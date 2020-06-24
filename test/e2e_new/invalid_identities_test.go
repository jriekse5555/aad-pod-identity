// +build e2e_new

package e2e_new

import (
	"fmt"

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	"github.com/Azure/aad-pod-identity/test/e2e_new/framework/azureassignedidentity"
	"github.com/Azure/aad-pod-identity/test/e2e_new/framework/azureidentity"
	"github.com/Azure/aad-pod-identity/test/e2e_new/framework/azureidentitybinding"
	"github.com/Azure/aad-pod-identity/test/e2e_new/framework/identityvalidator"
	"github.com/Azure/aad-pod-identity/test/e2e_new/framework/namespace"

	. "github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("[PR] When deploying invalid identities", func() {
	var (
		specName              = "invalid-identities"
		ns                    *corev1.Namespace
		azureIdentities       = make([]*aadpodv1.AzureIdentity, 3)
		azureIdentityBindings = make([]*aadpodv1.AzureIdentityBinding, 3)
	)

	BeforeEach(func() {
		ns = namespace.Create(namespace.CreateInput{
			Creator: kubeClient,
			Name:    specName,
		})

		for i, identityName := range []string{keyvaultIdentity, "invalid-identity-1", "invalid-identity-2"} {
			azureIdentities[i] = azureidentity.Create(azureidentity.CreateInput{
				Creator:      kubeClient,
				Config:       config,
				AzureClient:  azureClient,
				Name:         identityName,
				Namespace:    ns.Name,
				IdentityType: aadpodv1.UserAssignedMSI,
				IdentityName: identityName,
			})

			azureIdentityBindings[i] = azureidentitybinding.Create(azureidentitybinding.CreateInput{
				Creator:           kubeClient,
				Name:              fmt.Sprintf("%s-binding", identityName),
				Namespace:         ns.Name,
				AzureIdentityName: identityName,
				Selector:          fmt.Sprintf("%s-selector", identityName),
			})
		}
	})

	AfterEach(func() {
		namespace.Delete(namespace.DeleteInput{
			Deleter:   kubeClient,
			Getter:    kubeClient,
			Namespace: ns,
		})

		azureassignedidentity.WaitForLen(azureassignedidentity.WaitForLenInput{
			Lister: kubeClient,
			Len:    0,
		})
	})

	It("should assign the valid identity to the node when batch assigning identities that contain invalid identities", func() {
		identityValidators := identityvalidator.CreateBatch(identityvalidator.CreateBatchInput{
			Creator:          kubeClient,
			Config:           config,
			Namespace:        ns.Name,
			IdentityBindings: azureIdentityBindings,
		})

		azureassignedidentity.WaitForLen(azureassignedidentity.WaitForLenInput{
			Lister: kubeClient,
			Len:    3,
		})

		for i, tc := range []struct {
			stateToWaitFor string
			noError        bool
		}{
			{
				stateToWaitFor: aadpodv1.AssignedIDAssigned,
				noError:        true,
			},
			{
				stateToWaitFor: aadpodv1.AssignedIDCreated,
			},
			{
				stateToWaitFor: aadpodv1.AssignedIDCreated,
			},
		} {
			azureassignedidentity.Wait(azureassignedidentity.WaitInput{
				Getter:            kubeClient,
				PodName:           identityValidators[i].Name,
				Namespace:         ns.Name,
				AzureIdentityName: azureIdentities[i].Name,
				StateToWaitFor:    tc.stateToWaitFor,
			})

			if tc.noError {
				identityvalidator.Validate(identityvalidator.ValidateInput{
					Getter:           kubeClient,
					Config:           config,
					KubeconfigPath:   kubeconfigPath,
					PodName:          identityValidators[i].Name,
					Namespace:        ns.Name,
					IdentityClientID: azureIdentities[i].Spec.ClientID,
				})
			}
		}
	})

	It("should not unassign existing identity if we assign invalid identities", func() {
		validIdentityValidator := identityvalidator.Create(identityvalidator.CreateInput{
			Creator:         kubeClient,
			Config:          config,
			Namespace:       ns.Name,
			IdentityBinding: azureIdentityBindings[0].Spec.Selector,
		})

		azureassignedidentity.Wait(azureassignedidentity.WaitInput{
			Getter:            kubeClient,
			PodName:           validIdentityValidator.Name,
			Namespace:         ns.Name,
			AzureIdentityName: azureIdentities[0].Name,
			StateToWaitFor:    aadpodv1.AssignedIDAssigned,
		})

		identityvalidator.Validate(identityvalidator.ValidateInput{
			Getter:           kubeClient,
			Config:           config,
			KubeconfigPath:   kubeconfigPath,
			PodName:          validIdentityValidator.Name,
			Namespace:        ns.Name,
			IdentityClientID: azureIdentities[0].Spec.ClientID,
		})

		invalidIdentityValidators := identityvalidator.CreateBatch(identityvalidator.CreateBatchInput{
			Creator:          kubeClient,
			Config:           config,
			Namespace:        ns.Name,
			IdentityBindings: azureIdentityBindings[1:],
		})

		for i := 0; i < 2; i++ {
			azureassignedidentity.Wait(azureassignedidentity.WaitInput{
				Getter:            kubeClient,
				PodName:           invalidIdentityValidators[i].Name,
				Namespace:         ns.Name,
				AzureIdentityName: azureIdentities[i+1].Name,
				StateToWaitFor:    aadpodv1.AssignedIDCreated,
			})
		}

		identityvalidator.Validate(identityvalidator.ValidateInput{
			Getter:           kubeClient,
			Config:           config,
			KubeconfigPath:   kubeconfigPath,
			PodName:          validIdentityValidator.Name,
			Namespace:        ns.Name,
			IdentityClientID: azureIdentities[0].Spec.ClientID,
		})
	})
})