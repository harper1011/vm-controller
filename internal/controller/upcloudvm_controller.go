package controller

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	upCloudClient "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
	v1alpha1 "github.com/harper1011/vm-controller/api/v1alpha1"
)

// UpCloudVMReconciler reconciles a UpCloudVM object
type UpCloudVMReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger logr.Logger
}

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state specified by the user.
// This function handles the creation, updating, and deletion of UpCloud VMs.
// For more details, check Reconcile and its result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *UpCloudVMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the UpCloudVM resource
	var upCloudVM v1alpha1.UpCloudVM
	if err := r.Get(ctx, req.NamespacedName, &upCloudVM); err != nil {
		if apiError.IsNotFound(err) {
			// The resource was deleted
			logger.Info("UpCloudVM resource not found. skip...")
			return ctrl.Result{}, nil
		}
		// Error fetching the resource, requeue the request
		logger.Error(err, "Failed to get UpCloudVM")
		return ctrl.Result{}, err
	}

	// Initialize the UpCloud API client
	err, svc := r.getservice()
	if err != nil {
		return ctrl.Result{}, err
	}
	// Handle deletion logic
	if !upCloudVM.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("Deleting UpCloud VM")
		if err := r.deleteUpCloudVM(svc, &upCloudVM); err != nil {
			logger.Error(err, "Failed to delete UpCloud VM")
			return ctrl.Result{}, err
		}
	}

	// Add finalizer for this CR
	if !containsString(upCloudVM.GetFinalizers(), "upcloud.finalizer") {
		if err := r.addFinalizer(ctx, &upCloudVM); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create or update the UpCloud VM
	if upCloudVM.Status.VMID == "" {
		// Call UpCloud API to create a new VM
		logger.Info("Creating new UpCloud VM")
		vmID, ip, err := r.createUpCloudVM(svc, &upCloudVM)
		if err != nil {
			logger.Error(err, "Failed to create UpCloud VM")
			return ctrl.Result{}, err
		}

		// Update the status with VM details
		upCloudVM.Status.VMID = vmID
		upCloudVM.Status.IPAddress = ip
		upCloudVM.Status.State = "Running"
		if err := r.Status().Update(ctx, &upCloudVM); err != nil {
			logger.Error(err, "Failed to update UpCloudVM status")
			return ctrl.Result{}, err
		}
	} else {
		// Check and update the existing UpCloud VM if needed
		logger.Info("Checking for updates to UpCloud VM")
		err := r.updateUpCloudVM(svc, &upCloudVM)
		if err != nil {
			logger.Error(err, "Failed to update UpCloud VM")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getservice initializes the UpCloud API client
func (r *UpCloudVMReconciler) getservice() (error, *service.Service) {
	username := os.Getenv("UPCLOUD_USERNAME")
	password := os.Getenv("UPCLOUD_PASSWORD")

	if len(username) == 0 {
		fmt.Fprintln(os.Stderr, "Username must be specified")
		return errors.New("Username must be specified"), nil
	}

	if len(password) == 0 {
		fmt.Fprintln(os.Stderr, "Password must be specified")
		return errors.New("Password must be specified"), nil
	}
	c := upCloudClient.New(username, password)
	return nil, service.New(c)
}

// add Finalizer to resource
func (r *UpCloudVMReconciler) addFinalizer(ctx context.Context, vm *v1alpha1.UpCloudVM) error {
	vm.SetFinalizers(append(vm.GetFinalizers(), "upcloud.finalizer"))
	if err := r.Update(ctx, vm); err != nil {
		return err
	}
	return nil
}

// createUpCloudVM calls the UpCloud API to create a new VM
func (r *UpCloudVMReconciler) createUpCloudVM(svc *service.Service, vm *v1alpha1.UpCloudVM) (string, string, error) {
	// Use the UpCloud API to create a new VM
	serverDetails, err := svc.CreateServer(context.Background(), &request.CreateServerRequest{
		Title: vm.Name,
		Plan:  vm.Spec.Plan,
		Zone:  vm.Spec.Zone,
		StorageDevices: []request.CreateServerStorageDevice{
			{
				Action:  "clone",
				Storage: vm.Spec.Template,
				Size:    vm.Spec.Storage,
				Tier:    "maxiops",
			},
		},
		CoreNumber:   vm.Spec.CPU,
		MemoryAmount: vm.Spec.Memory,
		// Add more parameters as needed
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create UpCloud VM: %w", err)
	}

	return serverDetails.UUID, serverDetails.IPAddresses[0].Address, nil
}

// updateUpCloudVM updates the UpCloud VM based on the changes in the Spec
func (r *UpCloudVMReconciler) updateUpCloudVM(svc *service.Service, vm *v1alpha1.UpCloudVM) error {
	// Implement logic to update the UpCloud VM based on changes in the Spec
	// For example, adjust CPU, memory, etc.
	return nil
}

// deleteUpCloudVM deletes the UpCloud VM
func (r *UpCloudVMReconciler) deleteUpCloudVM(svc *service.Service, vm *v1alpha1.UpCloudVM) error {
	if vm.Status.VMID == "" {
		return nil
	}

	err := svc.DeleteServerAndStorages(context.Background(), &request.DeleteServerAndStoragesRequest{
		UUID: vm.Status.VMID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete UpCloud VM: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpCloudVMReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.UpCloudVM{}).
		Complete(r)
}

// Helper functions
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}
