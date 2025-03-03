package keycloakclient

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	pkgErrors "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	keycloakApi "github.com/epam/edp-keycloak-operator/api/v1/v1"
	"github.com/epam/edp-keycloak-operator/controllers/helper"
	"github.com/epam/edp-keycloak-operator/controllers/keycloakclient/chain"
	"github.com/epam/edp-keycloak-operator/pkg/client/keycloak"
)

type Helper interface {
	SetFailureCount(fc helper.FailureCountable) time.Duration
	TryToDelete(ctx context.Context, obj helper.Deletable, terminator helper.Terminator, finalizer string) (isDeleted bool, resultErr error)
	GetScheme() *runtime.Scheme
	CreateKeycloakClientForRealm(ctx context.Context, realm *keycloakApi.KeycloakRealm) (keycloak.Client, error)
	UpdateStatus(obj client.Object) error
	GetOrCreateRealmOwnerRef(object helper.RealmChild, objectMeta *v1.ObjectMeta) (*keycloakApi.KeycloakRealm, error)
}

const (
	Fail                                = "FAIL"
	keyCloakClientOperatorFinalizerName = "keycloak.client.operator.finalizer.name"
)

func NewReconcileKeycloakClient(client client.Client, log logr.Logger, helper Helper) *ReconcileKeycloakClient {
	return &ReconcileKeycloakClient{
		client: client,
		helper: helper,
		log:    log.WithName("keycloak-client"),
		chain:  chain.Make(helper.GetScheme(), client, log.WithName("chain").WithName("keycloak-client")),
	}
}

// ReconcileKeycloakClient reconciles a KeycloakClient object.
type ReconcileKeycloakClient struct {
	client                  client.Client
	helper                  Helper
	log                     logr.Logger
	chain                   chain.Element
	successReconcileTimeout time.Duration
}

func (r *ReconcileKeycloakClient) SetupWithManager(mgr ctrl.Manager, successReconcileTimeout time.Duration) error {
	r.successReconcileTimeout = successReconcileTimeout

	pred := predicate.Funcs{
		UpdateFunc: helper.IsFailuresUpdated,
	}

	err := ctrl.NewControllerManagedBy(mgr).
		For(&keycloakApi.KeycloakClient{}, builder.WithPredicates(pred)).
		Complete(r)
	if err != nil {
		return fmt.Errorf("failed to setup KeycloakClient controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=v1.edp.epam.com,namespace=placeholder,resources=keycloakclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=v1.edp.epam.com,namespace=placeholder,resources=keycloakclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=v1.edp.epam.com,namespace=placeholder,resources=keycloakclients/finalizers,verbs=update

// Reconcile is a loop for reconciling KeycloakClient object.
func (r *ReconcileKeycloakClient) Reconcile(ctx context.Context, request reconcile.Request) (result reconcile.Result, resultErr error) {
	log := r.log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	log.Info("Reconciling KeycloakClient")

	var instance keycloakApi.KeycloakClient
	if err := r.client.Get(ctx, request.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return
		}

		resultErr = err

		return
	}

	if err := r.tryReconcile(ctx, &instance); err != nil {
		instance.Status.Value = err.Error()
		result.RequeueAfter = r.helper.SetFailureCount(&instance)

		log.Error(err, "an error has occurred while handling keycloak client", "name", request.Name)
	} else {
		helper.SetSuccessStatus(&instance)
		result.RequeueAfter = r.successReconcileTimeout
	}

	if err := r.helper.UpdateStatus(&instance); err != nil {
		resultErr = pkgErrors.Wrap(err, "unable to update status")
	}

	return
}

func (r *ReconcileKeycloakClient) tryReconcile(ctx context.Context, keycloakClient *keycloakApi.KeycloakClient) error {
	realm, err := r.getOrCreateRealmOwner(keycloakClient)
	if err != nil {
		return pkgErrors.Wrap(err, "unable to get realm for client")
	}

	kClient, err := r.helper.CreateKeycloakClientForRealm(ctx, realm)
	if err != nil {
		return pkgErrors.Wrap(err, "unable to create keycloak adapter client")
	}

	if err := r.chain.Serve(ctx, keycloakClient, kClient); err != nil {
		return pkgErrors.Wrap(err, "error during kc chain")
	}

	if _, err := r.helper.TryToDelete(ctx, keycloakClient, makeTerminator(keycloakClient.Status.ClientID,
		keycloakClient.Spec.TargetRealm, kClient, r.log.WithName("kclient-term")),
		keyCloakClientOperatorFinalizerName); err != nil {
		return pkgErrors.Wrap(err, "unable to delete kc client")
	}

	return nil
}
