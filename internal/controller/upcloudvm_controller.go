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

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
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

const (
	UPCloudFinalizer = "upcloud.finalizer"
)

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state specified by the user.
// This function handles the creation, updating, and deletion of UpCloud VMs.
// For more details, check Reconcile and its result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *UpCloudVMReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Logger = log.FromContext(ctx)

	// Fetch the UpCloudVM resource
	var upCloudVM v1alpha1.UpCloudVM
	if err := r.Get(ctx, req.NamespacedName, &upCloudVM); err != nil {
		if apiError.IsNotFound(err) {
			r.Logger.Info("UpCloudVM resource not found. skip...")
			return ctrl.Result{}, nil
		}
		// Error fetching the resource, requeue the request
		r.Logger.Error(err, "Failed to get UpCloudVM")
		return ctrl.Result{}, err
	}
	// Initialize the UpCloud API client and get service object
	err, svc := r.getService()
	if err != nil {
		return ctrl.Result{}, err
	}

	// Handle deletion logic
	if !upCloudVM.ObjectMeta.DeletionTimestamp.IsZero() {
		r.Logger.Info("Deleting UpCloud VM")
		if err := r.deleteUpCloudVM(svc, &upCloudVM); err != nil {
			r.Logger.Error(err, "Failed to delete UpCloud VM")
			return ctrl.Result{}, err
		}
		// Remove Finalizer from VM deletion
		upCloudVM.ObjectMeta.Finalizers = removeString(upCloudVM.ObjectMeta.Finalizers, UPCloudFinalizer)
		if err := r.Update(ctx, &upCloudVM); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Create or update the UpCloud VM
	// Add finalizer for this CR
	if !containsString(upCloudVM.GetFinalizers(), UPCloudFinalizer) {
		if err := r.addFinalizer(ctx, &upCloudVM); err != nil {
			return ctrl.Result{}, err
		}
	}
	if upCloudVM.Status.VMID == "" {
		// Create a new VM
		r.Logger.Info("Creating new UpCloud VM")
		vmID, ip, err := r.createUpCloudVM(svc, &upCloudVM)
		if err != nil {
			r.Logger.Error(err, "Failed to create UpCloud VM")
			return ctrl.Result{}, err
		}
		upCloudVM.Status.VMID = vmID
		upCloudVM.Status.IPAddress = ip
		upCloudVM.Status.State = "Running"
		if err := r.Status().Update(ctx, &upCloudVM); err != nil {
			r.Logger.Error(err, "Failed to update UpCloudVM status")
			return ctrl.Result{}, err
		}
	} else {
		// Check and update the existing UpCloud VM
		r.Logger.Info("Updating to UpCloud VM")
		vmID, err := r.updateUpCloudVM(svc, &upCloudVM)
		upCloudVM.Status.VMID = vmID
		if err != nil {
			r.Logger.Error(err, "Failed to update UpCloud VM")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getService initializes the UpCloud API client
func (r *UpCloudVMReconciler) getService() (error, *service.Service) {
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
	svc := service.New(upCloudClient.New(username, password))

	// Following is some copied code from UpCloud Go SDK for error handling
	// https://github.com/UpCloudLtd/upcloud-go-api?tab=readme-ov-file#error-handling
	_, err := svc.GetAccount(context.Background())
	if err != nil {
		// `upcloud.Problem` is the error object returned by all of the `Service` methods.
		//  You can differentiate between generic connection errors (like the API not being reachable) and service errors, which are errors returned in the response body by the API;
		//	this is useful for gracefully recovering from certain types of errors;
		var problem *upcloud.Problem

		if errors.As(err, &problem) {
			fmt.Println(problem.Status)        // HTTP status code returned by the API
			fmt.Print(problem.Title)           // Short, human-readable description of the problem
			fmt.Println(problem.CorrelationID) // Unique string that identifies the request that caused the problem; note that this field is not always populated
			fmt.Println(problem.InvalidParams) // List of invalid request parameters

			for _, invalidParam := range problem.InvalidParams {
				fmt.Println(invalidParam.Name)   // Path to the request field that is invalid
				fmt.Println(invalidParam.Reason) // Human-readable description of the problem with that particular field
			}
			// You can also check against the specific api error codes to programatically react to certain situations.
			// Base `upcloud` package exports all the error codes that API can return.
			// You can check which error code is return in which situation in UpCloud API docs -> https://developers.upcloud.com/1.3
			if problem.ErrorCode() == upcloud.ErrCodeResourceAlreadyExists {
				fmt.Println("Looks like we don't need to create this")
			}
			// `upcloud.Problem` implements the Error interface, so you can also just use it as any other error
			fmt.Println(fmt.Errorf("we got an error from the UpCloud API: %w", problem))
		} else {
			// This means you got an error, but it does not come from the API itself. This can happen, for example, if you have some connection issues,
			// or if the UpCloud API is unreachable for some other reason
			fmt.Println("We got a generic error!")
		}
		return err, nil
	}
	return nil, svc
}

// add Finalizer to resource
func (r *UpCloudVMReconciler) addFinalizer(ctx context.Context, vm *v1alpha1.UpCloudVM) error {
	vm.SetFinalizers(append(vm.GetFinalizers(), UPCloudFinalizer))
	if err := r.Update(ctx, vm); err != nil {
		return err
	}
	return nil
}

// createUpCloudVM calls the UpCloud API to create a new VM
func (r *UpCloudVMReconciler) createUpCloudVM(svc *service.Service, vm *v1alpha1.UpCloudVM) (string, string, error) {
	// Use the UpCloud API to create a new VM
	ctx := context.Background()
	serverDetails, err := svc.CreateServer(ctx, &request.CreateServerRequest{
		Title:    vm.Name,
		Plan:     vm.Spec.Plan,
		Zone:     vm.Spec.Zone,
		TimeZone: vm.Spec.TimeZone,
		StorageDevices: []request.CreateServerStorageDevice{
			{
				Action:  "clone",
				Storage: vm.Spec.StorageTemplate,
				Title:   vm.Name,
				Size:    vm.Spec.StorageSize,
				Tier:    "maxiops",
			},
		},
		CoreNumber:   vm.Spec.CPU,
		MemoryAmount: vm.Spec.Memory,
		LoginUser:    vm.Spec.LoginUser,
		UserData:     vm.Spec.UserData,
		Networking: &request.CreateServerNetworking{
			Interfaces: []request.CreateServerInterface{
				{
					IPAddresses: []request.CreateServerIPAddress{
						{
							Family: upcloud.IPAddressFamilyIPv4,
						},
					},
					Type: upcloud.NetworkTypeUtility,
				},
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create UpCloud VM: %w", err)
	}

	serverDetails, err = svc.WaitForServerState(ctx, &request.WaitForServerStateRequest{
		UUID:         serverDetails.UUID,
		DesiredState: upcloud.ServerStateStarted,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to wait for server: %#v", err)
		return "", "", err
	}

	fmt.Printf("Created UpCloud VM: %#v\n", serverDetails)
	return serverDetails.UUID, serverDetails.IPAddresses[0].Address, nil
}

// updateUpCloudVM updates the UpCloud VM based on the changes in the Spec
func (r *UpCloudVMReconciler) updateUpCloudVM(svc *service.Service, vm *v1alpha1.UpCloudVM) (string, error) {
	ctx := context.Background()
	// Get existing VM details
	serverDetails, err := svc.GetServerDetails(ctx, &request.GetServerDetailsRequest{
		UUID: vm.Status.VMID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get UpCloud VM: %w", err)
	}

	// Update labels if VM name changed
	newLabelSlice := serverDetails.Labels
	existTitle := serverDetails.Title
	newTitle := vm.Name
	if existTitle != newTitle {
		newLabelSlice = append(newLabelSlice, upcloud.Label{Key: "title", Value: newTitle})
	}
	// Update VM server
	serverDetails, err = svc.ModifyServer(ctx, &request.ModifyServerRequest{
		Labels:       &newLabelSlice,
		UUID:         serverDetails.UUID,
		Title:        newTitle,
		Plan:         vm.Spec.Plan,
		Zone:         vm.Spec.Zone,
		CoreNumber:   vm.Spec.CPU,
		TimeZone:     vm.Spec.TimeZone,
		MemoryAmount: vm.Spec.Memory,
	})
	if err != nil {
		return "", fmt.Errorf("failed to modify UpCloud VM: %w", err)
	}
	// Wait updated VM server to be ready
	serverDetails, err = svc.WaitForServerState(ctx, &request.WaitForServerStateRequest{
		UUID:         serverDetails.UUID,
		DesiredState: upcloud.ServerStateStarted,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to wait for server: %#v", err)
		return "", err
	}

	fmt.Printf("Updated UpCloud VM: %#v\n", serverDetails)
	return serverDetails.UUID, nil
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
