// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package upgrader_test

import (
	"errors"
	"fmt"
	"time"

	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-service-broker/broker/services"
	"github.com/pivotal-cf/on-demand-service-broker/service"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader"
	"github.com/pivotal-cf/on-demand-service-broker/upgrader/fakes"
)

var _ = Describe("Upgrader", func() {
	const (
		serviceInstanceId = "serviceInstanceId"
	)

	var (
		actualErr            error
		fakeListener         *fakes.FakeListener
		brokerServicesClient *fakes.FakeBrokerServices
		instanceLister       *fakes.FakeInstanceLister
		upgraderBuilder      upgrader.Builder
		fakeSleeper          *fakes.FakeSleeper

		upgradeOperationAccepted = services.UpgradeOperation{
			Type: services.UpgradeAccepted,
		}
		lastOperationSucceeded  = brokerapi.LastOperation{State: brokerapi.Succeeded}
		lastOperationInProgress = brokerapi.LastOperation{State: brokerapi.InProgress}
	)

	BeforeEach(func() {
		fakeListener = new(fakes.FakeListener)
		brokerServicesClient = new(fakes.FakeBrokerServices)
		instanceLister = new(fakes.FakeInstanceLister)
		fakeSleeper = new(fakes.FakeSleeper)
		upgraderBuilder = upgrader.Builder{
			BrokerServices:        brokerServicesClient,
			ServiceInstanceLister: instanceLister,
			Listener:              fakeListener,
			PollingInterval:       10 * time.Second,
			AttemptLimit:          5,
			AttemptInterval:       60 * time.Second,
			MaxInFlight:           1,
			Canaries:              0,
			Sleeper:               fakeSleeper,
		}

		instanceLister.LatestInstanceInfoStub = func(i service.Instance) (service.Instance, error) {
			return i, nil
		}
	})

	Context("when upgrading one instance", func() {
		Context("and is successful", func() {
			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
				brokerServicesClient.UpgradeInstanceReturns(upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(
					brokerapi.LastOperation{
						State:       brokerapi.Succeeded,
						Description: "foo",
					}, nil)
			})

			It("returns the list of successful upgrades", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()

				Expect(instanceLister.InstancesCallCount()).To(Equal(1))
				Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(1))

				hasReportedInstanceUpgradeStarted(fakeListener, 1, serviceInstanceId, 1, 1)
				hasReportedInstanceUpgradeStartResult(fakeListener, services.UpgradeAccepted)
				hasReportedUpgraded(fakeListener, serviceInstanceId)
				Expect(actualErr).NotTo(HaveOccurred())

				hasReportedStarting(fakeListener, upgraderBuilder.MaxInFlight)
				hasReportedCanariesStarting(fakeListener, 0)
				hasReportedCanariesFinished(fakeListener, 0)
			})
		})

		Context("and it fails", func() {
			Context("to get a list of service instances", func() {
				BeforeEach(func() {
					instanceLister.InstancesReturns(nil, errors.New("bad status code"))
				})

				It("returns an error", func() {
					upgradeTool := upgrader.New(&upgraderBuilder)
					actualErr = upgradeTool.Upgrade()
					Expect(actualErr).To(MatchError("error listing service instances: bad status code"))
				})
			})

			Context("due to a malformed service instance guid", func() {
				BeforeEach(func() {
					instanceLister.InstancesReturns([]service.Instance{{GUID: "not a guid Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD"}}, nil)
					brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{}, errors.New("failed"))
				})

				It("returns an error", func() {
					upgradeTool := upgrader.New(&upgraderBuilder)
					actualErr = upgradeTool.Upgrade()
					Expect(actualErr).To(MatchError("Upgrade failed for service instance not a guid Q#$%#$%^&&*$%^#$FGRTYW${T:WED:AWSD)E@#PE{:QS:{QLWD: failed\n"))
				})
			})
		})
	})

	Context("when upgrading an instance is not instant", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

			brokerServicesClient.UpgradeInstanceReturns(upgradeOperationAccepted, nil)

			brokerServicesClient.LastOperationReturns(lastOperationInProgress, nil)
			brokerServicesClient.LastOperationReturnsOnCall(2, brokerapi.LastOperation{
				State: brokerapi.Succeeded,
			}, nil)
		})

		It("polls last operation until successful", func() {
			upgradeTool := upgrader.New(&upgraderBuilder)
			actualErr = upgradeTool.Upgrade()
			Expect(brokerServicesClient.LastOperationCallCount()).To(Equal(3))
			Expect(actualErr).NotTo(HaveOccurred())
			hasReportedUpgraded(fakeListener, serviceInstanceId)
			hasSlept(fakeSleeper, 0, upgraderBuilder.PollingInterval)
		})
	})

	Context("when the CF service instance has been deleted", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.InstanceNotFound,
			}, nil)
		})

		It("ignores the deleted instance", func() {
			upgradeTool := upgrader.New(&upgraderBuilder)
			actualErr = upgradeTool.Upgrade()
			Expect(actualErr).NotTo(HaveOccurred())

			hasReportedInstanceUpgradeStartResult(fakeListener, services.InstanceNotFound)
			hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 0, 0, 1)
			hasReportedFinished(fakeListener, 0, 0, 1, 0)
			hasReportedAttempts(fakeListener, 1, 5)
		})
	})

	Context("when the bosh deployment cannot be found", func() {
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.OrphanDeployment,
			}, nil)
		})

		It("detects one orphan instance", func() {
			upgradeTool := upgrader.New(&upgraderBuilder)
			actualErr = upgradeTool.Upgrade()
			Expect(actualErr).NotTo(HaveOccurred())

			hasReportedInstanceUpgradeStartResult(fakeListener, services.OrphanDeployment)
			hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 1, 0, 0, 0)
			hasReportedFinished(fakeListener, 1, 0, 0, 0)
			hasReportedAttempts(fakeListener, 1, 5)
		})
	})

	Context("when a plan change is triggered after the service instance list has been acquired", func() {
		It("uses the new plan for the upgrade", func() {
			const serviceInstanceId = "serviceInstanceId"
			instanceLister.InstancesReturnsOnCall(0, []service.Instance{
				{GUID: serviceInstanceId, PlanUniqueID: "plan-id-1"},
			}, nil)
			instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{
				GUID: serviceInstanceId, PlanUniqueID: "plan-id-2",
			}, nil)
			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.UpgradeAccepted,
			}, nil)
			brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)

			upgraderBuilder.MaxInFlight = 1
			upgradeTool := upgrader.New(&upgraderBuilder)

			actualErr = upgradeTool.Upgrade()

			instance := brokerServicesClient.UpgradeInstanceArgsForCall(0)
			Expect(instance.PlanUniqueID).To(Equal("plan-id-2"))

			Expect(actualErr).NotTo(HaveOccurred())
		})
	})

	Context("when fetching the latest info about an instance fails", func() {
		Context("with an unexpected error", func() {
			It("continues the upgrade using the previously fetched info", func() {
				const serviceInstanceId = "serviceInstanceId"
				instanceLister.InstancesReturnsOnCall(0, []service.Instance{
					{GUID: serviceInstanceId, PlanUniqueID: "plan-id-1"},
				}, nil)
				instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, errors.New("unexpected error"))
				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.UpgradeAccepted,
				}, nil)
				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)

				upgradeTool := upgrader.New(&upgraderBuilder)

				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).ToNot(HaveOccurred())
				Expect(fakeListener.FailedToRefreshInstanceInfoCallCount()).To(Equal(1))
			})
		})

		Context("with an InstanceNotFound error", func() {
			It("treats the service as a deleted instance", func() {
				const serviceInstanceId = "serviceInstanceId"
				instanceLister.InstancesReturnsOnCall(0, []service.Instance{
					{GUID: serviceInstanceId, PlanUniqueID: "plan-id-1"},
				}, nil)
				instanceLister.LatestInstanceInfoReturnsOnCall(0, service.Instance{}, service.InstanceNotFound)
				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.UpgradeAccepted,
				}, nil)
				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)

				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()

				Expect(actualErr).NotTo(HaveOccurred())
				hasReportedFinished(fakeListener, 0, 0, 1, 0)
				hasReportedInstanceUpgradeStartResult(fakeListener, services.InstanceNotFound)
			})
		})
	})

	Context("when an operation is in progress for a service instance", func() {
		Context("when the number of retries is within the limit", func() {
			const serviceInstanceId = "serviceInstanceId"
			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(3, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("retries until the upgrade request is accepted", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).NotTo(HaveOccurred())

				Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(4), "number of service requests")
				hasReportedInstanceUpgradeStartResult(
					fakeListener,
					services.OperationInProgress,
					services.OperationInProgress,
					services.OperationInProgress,
					services.UpgradeAccepted,
				)
				hasReportedRetries(fakeListener, 1, 1, 1, 0)
				hasReportedFinished(fakeListener, 0, 1, 0, 0)
				hasReportedAttempts(fakeListener, 4, 5)
			})
		})

		Context("when the attemptLimit is reached", func() {
			const serviceInstanceId = "serviceInstanceId"
			BeforeEach(func() {
				upgraderBuilder.AttemptLimit = 2
				instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(3, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("stops retrying when the attemptLimit is reached", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).To(MatchError(fmt.Errorf("The following instances could not be upgraded: service-instance_%s", serviceInstanceId)))

				Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(2), "number of service requests")
				hasReportedInstanceUpgradeStartResult(
					fakeListener,
					services.OperationInProgress,
					services.OperationInProgress,
				)
				hasReportedRetries(fakeListener, 1, 1)
				hasReportedFinished(fakeListener, 0, 0, 0, 1)
			})
		})
	})

	Context("when deletion is in progress for a service instance", func() {
		const serviceInstanceId = "serviceInstanceId"
		BeforeEach(func() {
			instanceLister.InstancesReturns([]service.Instance{{GUID: serviceInstanceId}}, nil)

			brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
				Type: services.OperationInProgress,
			}, nil)
			brokerServicesClient.UpgradeInstanceReturnsOnCall(3, services.UpgradeOperation{
				Type: services.OrphanDeployment,
			}, nil)

			brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
		})

		It("retries until an orphan is detected", func() {
			upgradeTool := upgrader.New(&upgraderBuilder)
			actualErr = upgradeTool.Upgrade()
			Expect(actualErr).NotTo(HaveOccurred())
			Expect(brokerServicesClient.UpgradeInstanceCallCount()).To(Equal(4), "number of service requests")

			hasReportedRetries(fakeListener, 1, 1, 1, 0)
			hasReportedOrphans(fakeListener, 0, 0, 0, 1)
			hasReportedFinished(fakeListener, 1, 0, 0, 0)
		})
	})

	Context("when upgrading multiple instances", func() {
		Context("successfully", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"
			serviceInstance4 := "serviceInstanceId4"
			serviceInstance5 := "serviceInstanceId5"
			serviceInstance6 := "serviceInstanceId6"
			upgradeTaskID1 := 123
			upgradeTaskID2 := 456
			upgradeTaskID3 := 789
			upgradeTaskID4 := 790
			upgradeTaskID5 := 791
			upgradeTaskID6 := 792

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID1),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID2),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID3),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(3, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID4),
				}, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("returns a report with all instances upgraded", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).NotTo(HaveOccurred())

				hasReportedStarting(fakeListener, upgraderBuilder.MaxInFlight)
				hasReportedInstancesToUpgrade(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
				hasReportedWaitingFor(fakeListener, map[string]int{serviceInstance1: upgradeTaskID1, serviceInstance2: upgradeTaskID2, serviceInstance3: upgradeTaskID3})
				hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
				hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 3, 0, 0)
				hasReportedFinished(fakeListener, 0, 3, 0, 0)

				Expect(fakeListener.InstanceUpgradeStartingCallCount()).To(Equal(3))

				for i := 1; i <= 3; i++ {
					_, index, total, _ := fakeListener.InstanceUpgradeStartingArgsForCall(i - 1)
					Expect(index).To(Equal(i), "number of instances upgraded")
					Expect(total).To(Equal(3), "total number of instances")
				}
			})

			Describe("canary upgrades", func() {
				var (
					si1Controller *processController
					si2Controller *processController
					si3Controller *processController
					si4Controller *processController
				)

				BeforeEach(func() {
					si1Controller = newProcessController("si1")
					si2Controller = newProcessController("si2")
					si3Controller = newProcessController("si3")
					si4Controller = newProcessController("si4")

					availableInstances := []service.Instance{
						{GUID: serviceInstance1},
						{GUID: serviceInstance2},
						{GUID: serviceInstance3},
						{GUID: serviceInstance4},
					}
					instanceLister.InstancesReturns(availableInstances, nil)

					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch instance.GUID {
						case serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case serviceInstance2:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						case serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID4),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					brokerServicesClient.LastOperationStub = func(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
						switch instance {
						case serviceInstance1:
							si1Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance2:
							si2Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance3:
							si3Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance4:
							si4Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						}
						return brokerapi.LastOperation{}, errors.New("unexpected instance GUID")
					}

				})

				It("does not error if no service instances found and canaries is 2 ", func() {
					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2
					instanceLister.InstancesReturns([]service.Instance{}, nil)

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					wg.Wait()

					Expect(actualErr).NotTo(HaveOccurred())
					hasReportedFinished(fakeListener, 0, 0, 0, 0)
				})

				It("upgrades the canary instances in parallel", func() {
					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller)
					expectToHaveNotStarted(si3Controller, si4Controller)

					allowToProceed(si1Controller, si2Controller)
					expectToHaveStarted(si3Controller, si4Controller)

					allowToProceed(si3Controller, si4Controller)

					wg.Wait()

					Expect(actualErr).NotTo(HaveOccurred())

					By("logging that start upgrading canaries")
					hasReportedCanariesStarting(fakeListener, 1)

					By("logging when the canaries finish upgrading")
					hasReportedCanariesFinished(fakeListener, 1)
				})

				It("upgrades the canary instances in parallel, respecting maxInFlight", func() {
					upgraderBuilder.MaxInFlight = 2
					upgraderBuilder.Canaries = 3

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller)
					expectToHaveNotStarted(si3Controller, si4Controller)

					allowToProceed(si1Controller, si2Controller)

					expectToHaveStarted(si3Controller)
					expectToHaveNotStarted(si4Controller)

					allowToProceed(si3Controller)

					expectToHaveStarted(si4Controller)
					allowToProceed(si4Controller)

					wg.Wait()

					Expect(actualErr).NotTo(HaveOccurred())
				})

				It("stops upgrading if one of the canaries fails to upgrade", func() {
					brokerServicesClient.LastOperationReturnsOnCall(0, brokerapi.LastOperation{
						State:       brokerapi.Failed,
						Description: "this didn't work",
					}, nil)

					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 1

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller)
					expectToHaveNotStarted(si2Controller, si3Controller, si4Controller)

					allowToProceed(si1Controller)

					expectToHaveNotStarted(si2Controller, si3Controller, si4Controller)

					wg.Wait()

					Expect(actualErr).To(MatchError(ContainSubstring("canaries didn't upgrade successfully")))
					hasReportedFailureFor(fakeListener, serviceInstance1)
				})

				It("picks another canary instance if one is busy", func() {
					busyCount := 0
					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch guid := instance.GUID; {
						case guid == serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case guid == serviceInstance2 && busyCount == 0:
							busyCount++
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.OperationInProgress,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance2 && busyCount == 1:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil

						case guid == serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller, si3Controller)
					expectToHaveNotStarted(si4Controller)

					allowToProceed(si1Controller)
					expectToHaveNotStarted(si4Controller)

					allowToProceed(si3Controller)
					expectToHaveStarted(si4Controller)

					expectToHaveNotStarted(si2Controller)
					allowToProceed(si4Controller)

					expectToHaveStarted(si2Controller)

					hasReportedRetries(fakeListener, 1)

					allowToProceed(si2Controller)

					wg.Wait()
					Expect(actualErr).ToNot(HaveOccurred())
				})

				It("handles deleted instance in canary selection", func() {
					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch guid := instance.GUID; {
						case guid == serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case guid == serviceInstance2:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.InstanceNotFound,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil

						case guid == serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller, si3Controller)
					expectToHaveNotStarted(si4Controller)

					allowToProceed(si1Controller, si3Controller)

					expectToHaveStarted(si4Controller)

					allowToProceed(si4Controller)

					wg.Wait()
					Expect(actualErr).ToNot(HaveOccurred())

					hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance3, serviceInstance4)
					hasReportedFinished(fakeListener, 0, 3, 1, 0)
				})

				It("fails after reaching the attempt limit threshold", func() {
					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch guid := instance.GUID; {
						case guid == serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case guid == serviceInstance2:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.OperationInProgress,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						case guid == serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID4),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					upgraderBuilder.MaxInFlight = 4
					upgraderBuilder.Canaries = 4
					upgraderBuilder.AttemptInterval = time.Millisecond
					upgraderBuilder.AttemptLimit = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller, si3Controller, si4Controller)
					allowToProceed(si1Controller, si3Controller, si4Controller)

					By("retrying the busy instance")
					expectToHaveStarted(si2Controller)

					wg.Wait()

					By("erring as it reached the retry limit")
					Expect(actualErr).To(HaveOccurred())
					Expect(actualErr).To(MatchError(ContainSubstring(
						"canaries didn't upgrade successfully: attempted to upgrade 4 canaries, but only found 3 instances not already in use by another BOSH task.",
					)))

					hasReportedRetries(fakeListener, 1, 1)
					hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 3, 1, 0)
					hasReportedProgress(fakeListener, 1, upgraderBuilder.AttemptInterval, 0, 3, 1, 0)
					hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance3, serviceInstance4)
				})

				It("retries busy instance after all canaries pass", func() {
					busyCount := 0
					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch guid := instance.GUID; {
						case guid == serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case guid == serviceInstance2 && busyCount == 0:
							busyCount++
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.OperationInProgress,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance2 && busyCount == 1:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil

						case guid == serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 1

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller)
					expectToHaveNotStarted(si2Controller, si3Controller, si4Controller)

					allowToProceed(si1Controller)

					expectToHaveStarted(si2Controller, si3Controller, si4Controller)

					allowToProceed(si3Controller, si4Controller)

					expectToHaveStarted(si2Controller)

					allowToProceed(si2Controller)

					wg.Wait()

					hasReportedRetries(fakeListener, 1, 0)
					Expect(actualErr).ToNot(HaveOccurred())
				})

				It("retries busy instance after all canaries pass", func() {
					states := []services.UpgradeOperationType{
						services.UpgradeAccepted,
						services.OperationInProgress,
						services.OperationInProgress,
						services.OperationInProgress,
					}
					mutex := sync.Mutex{}
					getState := func(i int) services.UpgradeOperationType {
						mutex.Lock()
						defer mutex.Unlock()

						return states[i]
					}

					setState := func(i int, t services.UpgradeOperationType) {
						mutex.Lock()
						defer mutex.Unlock()
						states[i] = t
					}

					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						var i int
						switch guid := instance.GUID; {
						case guid == serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case guid == serviceInstance2:
							i = 1
							si2Controller.NotifyStart()
						case guid == serviceInstance3:
							i = 2
							si3Controller.NotifyStart()
						case guid == serviceInstance4:
							i = 3
							si4Controller.NotifyStart()
						default:
							return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
						}
						return services.UpgradeOperation{
							Type: getState(i),
							Data: upgradeResponse(upgradeTaskID1),
						}, nil
					}

					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2
					upgraderBuilder.AttemptLimit = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					// Retry attempt 1: Canaries
					expectToHaveStarted(si1Controller, si2Controller, si3Controller, si4Controller)
					//set si2 to be not busy on next call
					setState(1, services.UpgradeAccepted)
					allowToProceed(si1Controller)

					// Retry attempt 2: Canaries
					expectToHaveStarted(si2Controller)
					expectToHaveNotStarted(si3Controller, si4Controller)
					// set si3 to be not busy on next call
					setState(2, services.UpgradeAccepted)
					allowToProceed(si2Controller)
					// Canaries completed

					// Retry attempt 1: Upgrade
					expectToHaveStarted(si3Controller, si4Controller)
					// set si4 to be not busy on next call
					setState(3, services.UpgradeAccepted)
					allowToProceed(si3Controller)

					// Retry attemp 2 : Upgrade
					expectToHaveStarted(si4Controller)
					allowToProceed(si4Controller)

					wg.Wait()

					Expect(actualErr).ToNot(HaveOccurred())

					expectCanariesRetryCallCount := 2
					Expect(fakeListener.RetryCanariesAttemptCallCount()).To(Equal(expectCanariesRetryCallCount))
					expectedCanariesParams := [][]int{
						{1, 2, 2},
						{2, 2, 1},
					}
					for i := 0; i < expectCanariesRetryCallCount; i++ {
						a, t, c := fakeListener.RetryCanariesAttemptArgsForCall(i)
						Expect(a).To(Equal(expectedCanariesParams[i][0]))
						Expect(t).To(Equal(expectedCanariesParams[i][1]))
						Expect(c).To(Equal(expectedCanariesParams[i][2]))
					}
					expectedCallCount := 2
					Expect(fakeListener.RetryAttemptCallCount()).To(Equal(expectedCallCount))
					expectedParams := [][]int{
						{1, 2},
						{2, 2},
					}
					for i := 0; i < expectedCallCount; i++ {
						a, t := fakeListener.RetryAttemptArgsForCall(i)
						Expect(a).To(Equal(expectedParams[i][0]))
						Expect(t).To(Equal(expectedParams[i][1]))
					}

					expectedInstanceCounts := [][]int{
						{1, 4, 1},
						{2, 4, 1},
						{2, 4, 1},
						{2, 4, 1},
						{2, 4, 1},
						{3, 4, 0},
						{4, 4, 0},
						{4, 4, 0},
					}
					for i := 0; i < fakeListener.InstanceUpgradeStartingCallCount(); i++ {
						_, index, total, isCanary := fakeListener.InstanceUpgradeStartingArgsForCall(i)
						Expect(index).To(Equal(expectedInstanceCounts[i][0]), fmt.Sprintf("Current instance index; i = %d", i))
						Expect(total).To(Equal(expectedInstanceCounts[i][1]), "Total pending instances")
						Expect(isCanary).To(Equal(expectedInstanceCounts[i][2] == 1), "Total pending instances")
					}
				})
			})

			Describe("upgrade with canary instances an multiple rounds", func() {
				var (
					si1Controller *processController
					si2Controller *processController
					si3Controller *processController
					si4Controller *processController
					si5Controller *processController
					si6Controller *processController
				)

				BeforeEach(func() {
					si1Controller = newProcessController("si1")
					si2Controller = newProcessController("si2")
					si3Controller = newProcessController("si3")
					si4Controller = newProcessController("si4")
					si5Controller = newProcessController("si5")
					si6Controller = newProcessController("si6")

					availableInstances := []service.Instance{
						{GUID: serviceInstance1},
						{GUID: serviceInstance2},
						{GUID: serviceInstance3},
						{GUID: serviceInstance4},
						{GUID: serviceInstance5},
						{GUID: serviceInstance6},
					}
					instanceLister.InstancesReturns(availableInstances, nil)

					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch instance.GUID {
						case serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case serviceInstance2:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						case serviceInstance4:
							si4Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID4),
							}, nil
						case serviceInstance5:
							si5Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID5),
							}, nil
						case serviceInstance6:
							si6Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID6),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}

					brokerServicesClient.LastOperationStub = func(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
						switch instance {
						case serviceInstance1:
							si1Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance2:
							si2Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance3:
							si3Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance4:
							si4Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance5:
							si5Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						case serviceInstance6:
							si6Controller.WaitForSignalToProceed()
							return brokerapi.LastOperation{
								State: brokerapi.Succeeded,
							}, nil
						}
						return brokerapi.LastOperation{}, errors.New("unexpected instance GUID")
					}
				})

				It("Complete the upgrade in multiple rounds", func() {
					upgraderBuilder.MaxInFlight = 3
					upgraderBuilder.Canaries = 2

					upgradeTool := upgrader.New(&upgraderBuilder)

					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer GinkgoRecover()
						defer wg.Done()
						actualErr = upgradeTool.Upgrade()
					}()

					expectToHaveStarted(si1Controller, si2Controller)
					expectToHaveNotStarted(si3Controller, si4Controller, si5Controller, si6Controller)

					allowToProceed(si1Controller, si2Controller)

					expectToHaveStarted(si3Controller, si4Controller, si5Controller)
					expectToHaveNotStarted(si6Controller)

					allowToProceed(si3Controller, si4Controller, si5Controller)

					expectToHaveStarted(si6Controller)
					allowToProceed(si6Controller)

					wg.Wait()

					Expect(actualErr).NotTo(HaveOccurred())

					expectedCanariesCallCount := 1
					Expect(fakeListener.RetryCanariesAttemptCallCount()).To(Equal(expectedCanariesCallCount))
					expectedCanariesParams := [][]int{
						{1, 5, 2},
					}
					for i := 0; i < expectedCanariesCallCount; i++ {
						a, t, c := fakeListener.RetryCanariesAttemptArgsForCall(i)
						Expect(a).To(Equal(expectedCanariesParams[i][0]))
						Expect(t).To(Equal(expectedCanariesParams[i][1]))
						Expect(c).To(Equal(expectedCanariesParams[i][2]))
					}

					expectedCallCount := 1
					Expect(fakeListener.RetryAttemptCallCount()).To(Equal(expectedCallCount))
					expectedParams := [][]int{
						{1, 5},
						{1, 5},
					}
					for i := 0; i < expectedCallCount; i++ {
						a, t := fakeListener.RetryAttemptArgsForCall(i)
						Expect(a).To(Equal(expectedParams[i][0]))
						Expect(t).To(Equal(expectedParams[i][1]))
					}
				})
			})

			Describe("parallel upgrades", func() {
				var (
					si1Controller *processController
					si2Controller *processController
					si3Controller *processController
				)

				BeforeEach(func() {
					si1Controller = newProcessController("si1")
					si2Controller = newProcessController("si2")
					si3Controller = newProcessController("si3")

					brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
						switch instance.GUID {
						case serviceInstance1:
							si1Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID1),
							}, nil
						case serviceInstance2:
							si2Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID2),
							}, nil
						case serviceInstance3:
							si3Controller.NotifyStart()
							return services.UpgradeOperation{
								Type: services.UpgradeAccepted,
								Data: upgradeResponse(upgradeTaskID3),
							}, nil
						}
						return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
					}
				})

				Context("when max_in_flight is 3", func() {
					BeforeEach(func() {
						upgraderBuilder.MaxInFlight = 3
					})

					It("starts 3 upgrade processes simultaneously and returns a report with all instances upgraded", func() {
						upgradeTool := upgrader.New(&upgraderBuilder)

						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							defer GinkgoRecover()
							defer wg.Done()
							actualErr = upgradeTool.Upgrade()
						}()

						expectToHaveStarted(si1Controller, si2Controller, si3Controller)
						allowToProceed(si1Controller, si2Controller, si3Controller)

						wg.Wait()

						Expect(actualErr).NotTo(HaveOccurred())

						hasReportedStarting(fakeListener, upgraderBuilder.MaxInFlight)
						hasReportedInstancesToUpgrade(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
						hasReportedWaitingFor(fakeListener, map[string]int{serviceInstance1: upgradeTaskID1, serviceInstance2: upgradeTaskID2, serviceInstance3: upgradeTaskID3})
						hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
						hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 3, 0, 0)
						hasReportedFinished(fakeListener, 0, 3, 0, 0)
					})
				})

				Context("when max_in_flight is 2", func() {
					BeforeEach(func() {
						upgraderBuilder.MaxInFlight = 2
					})

					It("starts 2 upgrade processes simultaneously and the 3rd once one is finished", func() {
						brokerServicesClient.LastOperationStub = func(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
							switch instance {
							case serviceInstance1:
								si1Controller.WaitForSignalToProceed()
								return brokerapi.LastOperation{
									State: brokerapi.Succeeded,
								}, nil
							case serviceInstance2:
								si2Controller.WaitForSignalToProceed()
								return brokerapi.LastOperation{
									State: brokerapi.Succeeded,
								}, nil
							case serviceInstance3:
								si3Controller.WaitForSignalToProceed()
								return brokerapi.LastOperation{
									State: brokerapi.Succeeded,
								}, nil
							}
							return brokerapi.LastOperation{}, errors.New("unexpected instance GUID")
						}

						upgradeTool := upgrader.New(&upgraderBuilder)

						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							defer GinkgoRecover()
							defer wg.Done()
							actualErr = upgradeTool.Upgrade()
						}()

						expectToHaveStarted(si1Controller, si2Controller)
						expectToHaveNotStarted(si3Controller)

						allowToProceed(si1Controller, si2Controller)
						expectToHaveStarted(si3Controller)

						allowToProceed(si3Controller)

						wg.Wait()

						Expect(actualErr).NotTo(HaveOccurred())

						hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
						hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 3, 0, 0)
						hasReportedFinished(fakeListener, 0, 3, 0, 0)
					})

					It("never runs 3rd upgrade if 1st fails", func() {
						brokerServicesClient.LastOperationReturnsOnCall(0, brokerapi.LastOperation{
							State:       brokerapi.Failed,
							Description: "this didn't work",
						}, nil)

						allowToProceed(si3Controller)

						upgradeTool := upgrader.New(&upgraderBuilder)

						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							defer GinkgoRecover()
							defer wg.Done()
							actualErr = upgradeTool.Upgrade()
						}()

						expectToHaveStarted(si1Controller, si2Controller)
						expectToHaveNotStarted(si3Controller)

						allowToProceed(si1Controller)
						expectToHaveNotStarted(si3Controller)
						allowToProceed(si2Controller)
						expectToHaveNotStarted(si3Controller)

						wg.Wait()

						Expect(actualErr).To(MatchError(fmt.Sprintf(
							"[%s] Upgrade failed: bosh task id %d: this didn't work",
							serviceInstance1,
							upgradeTaskID1,
						)))

						hasReportedUpgraded(fakeListener, serviceInstance2)
					})

					It("returns both error messages if two upgrades fail", func() {
						brokerServicesClient.LastOperationReturns(brokerapi.LastOperation{
							State:       brokerapi.Failed,
							Description: "this didn't work",
						}, nil)

						allowToProceed(si3Controller)

						upgradeTool := upgrader.New(&upgraderBuilder)

						var wg sync.WaitGroup
						wg.Add(1)
						go func() {
							defer GinkgoRecover()
							defer wg.Done()
							actualErr = upgradeTool.Upgrade()
						}()

						expectToHaveStarted(si1Controller, si2Controller)
						expectToHaveNotStarted(si3Controller)

						allowToProceed(si1Controller)
						expectToHaveNotStarted(si3Controller)
						allowToProceed(si2Controller)

						expectToHaveNotStarted(si3Controller)

						wg.Wait()

						Expect(actualErr).To(MatchError(fmt.Sprintf(
							"2 errors occurred:\n\n* [%s] Upgrade failed: bosh task id %d: this didn't work\n* [%s] Upgrade failed: bosh task id %d: this didn't work",
							serviceInstance1,
							upgradeTaskID1,
							serviceInstance2,
							upgradeTaskID2,
						)))

						hasReportedFailureFor(fakeListener, serviceInstance1, serviceInstance2)
						hasReportedFinished(fakeListener, 0, 0, 0, 0, serviceInstance1, serviceInstance2)
					})

					Context("when retries are required", func() {
						It("it retries a single busy upgrade only when all other upgrades have completed", func() {
							busyCount := 0
							si1Controller2 := newProcessController("si1")
							brokerServicesClient.UpgradeInstanceStub = func(instance service.Instance) (services.UpgradeOperation, error) {
								switch guid := instance.GUID; {
								case guid == serviceInstance1 && busyCount == 0:
									busyCount++
									si1Controller.NotifyStart()
									return services.UpgradeOperation{
										Type: services.OperationInProgress,
										Data: upgradeResponse(upgradeTaskID1),
									}, nil
								case guid == serviceInstance1 && busyCount == 1:
									si1Controller2.NotifyStart()
									return services.UpgradeOperation{
										Type: services.UpgradeAccepted,
										Data: upgradeResponse(upgradeTaskID1),
									}, nil
								case guid == serviceInstance2:
									si2Controller.NotifyStart()
									return services.UpgradeOperation{
										Type: services.UpgradeAccepted,
										Data: upgradeResponse(upgradeTaskID2),
									}, nil
								case guid == serviceInstance3:
									si3Controller.NotifyStart()
									return services.UpgradeOperation{
										Type: services.UpgradeAccepted,
										Data: upgradeResponse(upgradeTaskID3),
									}, nil
								}
								return services.UpgradeOperation{}, errors.New("unexpected instance GUID")
							}

							brokerServicesClient.LastOperationStub = func(instance string, operationData broker.OperationData) (brokerapi.LastOperation, error) {
								switch {
								case instance == serviceInstance1:
									si1Controller2.WaitForSignalToProceed()
									return brokerapi.LastOperation{
										State: brokerapi.Succeeded,
									}, nil
								case instance == serviceInstance2:
									si2Controller.WaitForSignalToProceed()
									return brokerapi.LastOperation{
										State: brokerapi.Succeeded,
									}, nil
								case instance == serviceInstance3:
									si3Controller.WaitForSignalToProceed()
									return brokerapi.LastOperation{
										State: brokerapi.Succeeded,
									}, nil
								}
								return brokerapi.LastOperation{}, errors.New("unexpected instance GUID")
							}

							upgradeTool := upgrader.New(&upgraderBuilder)

							var wg sync.WaitGroup
							wg.Add(1)
							go func() {
								defer GinkgoRecover()
								defer wg.Done()
								actualErr = upgradeTool.Upgrade()
							}()

							expectToHaveStarted(si1Controller, si2Controller, si3Controller)
							expectToHaveNotStarted(si1Controller2)

							allowToProceed(si2Controller)
							expectToHaveNotStarted(si1Controller2)

							allowToProceed(si3Controller)

							expectToHaveStarted(si1Controller2)

							allowToProceed(si1Controller2)

							wg.Wait()

							hasReportedRetries(fakeListener, 1, 0)
							hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 2, 1, 0)
							hasReportedProgress(fakeListener, 1, upgraderBuilder.AttemptInterval, 0, 3, 0, 0)
							hasReportedUpgraded(fakeListener, serviceInstance1, serviceInstance2, serviceInstance3)
							hasReportedFinished(fakeListener, 0, 3, 0, 0)
						})
					})
				})
			})
		})

		Context("and the second upgrade request fails", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{}, errors.New("upgrade failed"))

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("returns the upgrade request error", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				message := fmt.Sprintf(
					"Upgrade failed for service instance %s: upgrade failed\n",
					serviceInstance2,
				)
				Expect(actualErr).To(MatchError(message))
			})
		})

		Context("and the second upgrade operation fails", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"
			upgradeTaskID1 := 432
			upgradeTaskID2 := 987

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID1),
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.UpgradeAccepted,
					Data: upgradeResponse(upgradeTaskID2),
				}, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
				brokerServicesClient.LastOperationReturnsOnCall(1, brokerapi.LastOperation{
					State:       brokerapi.Failed,
					Description: "everything went wrong",
				}, nil)
			})

			It("reports the upgrade operation error", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				failureMessage := fmt.Sprintf(
					"[%s] Upgrade failed: bosh task id %d: everything went wrong",
					serviceInstance2,
					upgradeTaskID2,
				)
				Expect(actualErr).To(MatchError(failureMessage))

				hasReportedWaitingFor(fakeListener, map[string]int{serviceInstance1: upgradeTaskID1, serviceInstance2: upgradeTaskID2})
				hasReportedFailureFor(fakeListener, serviceInstance2)
			})
		})

		Context("and the second instance is orphaned", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(1, services.UpgradeOperation{
					Type: services.OrphanDeployment,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, upgradeOperationAccepted, nil)
				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("reports one orphaned instance", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).NotTo(HaveOccurred())
				hasReportedAttempts(fakeListener, 1, 5)
				hasReportedFinished(fakeListener, 1, 2, 0, 0)
			})
		})

		Context("and one has a BOSH operation in progress", func() {
			serviceInstance1 := "serviceInstanceId1"
			serviceInstance2 := "serviceInstanceId2"
			serviceInstance3 := "serviceInstanceId3"

			BeforeEach(func() {
				instanceLister.InstancesReturns([]service.Instance{
					{GUID: serviceInstance1},
					{GUID: serviceInstance2},
					{GUID: serviceInstance3},
				}, nil)

				brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturns(services.UpgradeOperation{
					Type: services.OperationInProgress,
				}, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(2, upgradeOperationAccepted, nil)
				brokerServicesClient.UpgradeInstanceReturnsOnCall(5, upgradeOperationAccepted, nil)

				brokerServicesClient.LastOperationReturns(lastOperationSucceeded, nil)
			})

			It("retries until all are upgraded", func() {
				upgradeTool := upgrader.New(&upgraderBuilder)
				actualErr = upgradeTool.Upgrade()
				Expect(actualErr).NotTo(HaveOccurred())

				upgradeServiceInstance2CallCount := 0
				for x := 0; x < brokerServicesClient.UpgradeInstanceCallCount(); x++ {
					instance := brokerServicesClient.UpgradeInstanceArgsForCall(x)
					if instance.GUID == serviceInstance2 {
						upgradeServiceInstance2CallCount++
					}
				}

				Expect(upgradeServiceInstance2CallCount).To(Equal(4), "number of service requests")
				hasReportedRetries(fakeListener, 1, 1, 1, 0)
				hasReportedFinished(fakeListener, 0, 3, 0, 0)
				hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 2, 1, 0)

				hasSlept(fakeSleeper, 0, upgraderBuilder.AttemptInterval)
			})
		})

		It("sleeps when the last operation reports In Progress", func() {
			serviceInstance1 := "serviceInstanceId1"

			instanceLister.InstancesReturns([]service.Instance{
				{GUID: serviceInstance1},
			}, nil)

			brokerServicesClient.UpgradeInstanceReturnsOnCall(0, upgradeOperationAccepted, nil)

			brokerServicesClient.LastOperationReturns(lastOperationInProgress, nil)
			brokerServicesClient.LastOperationReturnsOnCall(3, lastOperationSucceeded, nil)
			upgradeTool := upgrader.New(&upgraderBuilder)

			actualErr = upgradeTool.Upgrade()
			Expect(actualErr).NotTo(HaveOccurred())

			hasReportedRetries(fakeListener, 0)
			hasReportedFinished(fakeListener, 0, 1, 0, 0)
			hasReportedProgress(fakeListener, 0, upgraderBuilder.AttemptInterval, 0, 1, 0, 0)

			hasSlept(fakeSleeper, 0, upgraderBuilder.PollingInterval)
			hasSlept(fakeSleeper, 1, upgraderBuilder.PollingInterval)
			hasSlept(fakeSleeper, 2, upgraderBuilder.PollingInterval)
		})
	})
})

func upgradeResponse(taskId int) broker.OperationData {
	return broker.OperationData{BoshTaskID: taskId, OperationType: broker.OperationTypeUpgrade}
}

func hasReportedStarting(fakeListener *fakes.FakeListener, maxInFlight int) {
	Expect(fakeListener.StartingCallCount()).To(Equal(1))
	threads := fakeListener.StartingArgsForCall(0)
	Expect(threads).To(Equal(maxInFlight))
}

func hasReportedInstancesToUpgrade(fakeListener *fakes.FakeListener, instanceIds ...string) {
	Expect(fakeListener.InstancesToUpgradeCallCount()).To(Equal(1))
	Expect(fakeListener.InstancesToUpgradeArgsForCall(0)).To(Equal(makeInstanceMapFromIds(instanceIds)))
}

func hasReportedWaitingFor(fakeListener *fakes.FakeListener, instances map[string]int) {
	calls := fakeListener.WaitingForCallCount()
	Expect(calls).To(Equal(len(instances)))
	for i := 0; i < calls; i++ {
		instanceId, taskId := fakeListener.WaitingForArgsForCall(i)
		Expect(instances[instanceId]).To(Equal(taskId), "Task Id for "+instanceId)
	}
}

func hasReportedInstanceUpgradeStarted(fakeListener *fakes.FakeListener, callCount int, expectedInstance string, expectedIndex, expectedTotalInstances int) {
	Expect(fakeListener.InstanceUpgradeStartingCallCount()).To(
		Equal(callCount), "instance upgrade started call count",
	)

	actualInstance, actualIndex, actualTotalInstances, _ := fakeListener.InstanceUpgradeStartingArgsForCall(0)
	Expect(actualInstance).To(Equal(expectedInstance))
	Expect(actualIndex).To(Equal(expectedIndex), "expected index for instance upgrade started")
	Expect(actualTotalInstances).To(Equal(expectedTotalInstances), "expected total num of instances for instance upgrade started")
}

func hasReportedInstanceUpgradeStartResult(fakeListener *fakes.FakeListener, expectedStatuses ...services.UpgradeOperationType) {
	Expect(fakeListener.InstanceUpgradeStartResultCallCount()).To(
		Equal(len(expectedStatuses)), "instance upgrade start result call count",
	)

	for i, expectedStatus := range expectedStatuses {
		_, status := fakeListener.InstanceUpgradeStartResultArgsForCall(i)
		Expect(status).To(Equal(expectedStatus))
	}
}

func hasReportedUpgraded(fakeListener *fakes.FakeListener, expectedInstanceIds ...string) {
	hasReportedUpgradeStates(fakeListener, "success", expectedInstanceIds...)
}

func hasReportedFailureFor(fakeListener *fakes.FakeListener, expectedInstanceIds ...string) {
	hasReportedUpgradeStates(fakeListener, "failure", expectedInstanceIds...)
}

func hasReportedUpgradeStates(fakeListener *fakes.FakeListener, expectedStatus string, expectedInstanceIds ...string) {
	upgraded := make([]service.Instance, 0)
	for i := 0; i < fakeListener.InstanceUpgradedCallCount(); i++ {
		id, status := fakeListener.InstanceUpgradedArgsForCall(i)
		if status == expectedStatus {
			upgraded = append(upgraded, service.Instance{GUID: id})
		}
	}

	expectedInstances := makeInstanceMapFromIds(expectedInstanceIds)

	Expect(upgraded).To(ConsistOf(expectedInstances), "status="+expectedStatus)
}

func makeInstanceMapFromIds(expectedInstanceIds []string) []service.Instance {
	var expectedInstances []service.Instance
	for _, expectedInstanceId := range expectedInstanceIds {
		expectedInstances = append(expectedInstances, service.Instance{GUID: expectedInstanceId})
	}
	return expectedInstances
}

func hasSlept(fakeSleeper *fakes.FakeSleeper, callIndex int, expectedInterval time.Duration) {
	Expect(fakeSleeper.SleepCallCount()).To(BeNumerically(">", callIndex))
	Expect(fakeSleeper.SleepArgsForCall(callIndex)).To(Equal(expectedInterval))
}

func hasReportedRetries(fakeListener *fakes.FakeListener, expectedPendingInstancesCount ...int) {
	for i, expectedRetryCount := range expectedPendingInstancesCount {
		_, _, _, toRetryCount, _ := fakeListener.ProgressArgsForCall(i)
		Expect(toRetryCount).To(Equal(expectedRetryCount), "Retry count: "+string(i))
	}
}

func hasReportedOrphans(fakeListener *fakes.FakeListener, expectedOrphanCounts ...int) {
	for i, expectedOrphanCount := range expectedOrphanCounts {
		_, orphanCount, _, _, _ := fakeListener.ProgressArgsForCall(i)
		Expect(orphanCount).To(Equal(expectedOrphanCount), "Orphan count: "+string(i))
	}
}

func hasReportedProgress(fakeListener *fakes.FakeListener, callIndex int, expectedInterval time.Duration, expectedOrphans, expectedUpgraded, expectedToRetry, expectedDeleted int) {
	Expect(fakeListener.ProgressCallCount()).To(BeNumerically(">", callIndex))
	attemptInterval, orphanCount, upgradedCount, toRetryCount, deletedCount := fakeListener.ProgressArgsForCall(callIndex)
	Expect(attemptInterval).To(Equal(expectedInterval), "attempt interval")
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(toRetryCount).To(Equal(expectedToRetry), "to retry")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
}

func hasReportedFinished(fakeListener *fakes.FakeListener, expectedOrphans, expectedUpgraded, expectedDeleted, expectedCouldNotStart int, expectedFailedInstances ...string) {
	Expect(fakeListener.FinishedCallCount()).To(Equal(1))
	orphanCount, upgradedCount, deletedCount, couldNotStartCount, failedInstances := fakeListener.FinishedArgsForCall(0)
	Expect(orphanCount).To(Equal(expectedOrphans), "orphans")
	Expect(upgradedCount).To(Equal(expectedUpgraded), "upgraded")
	Expect(deletedCount).To(Equal(expectedDeleted), "deleted")
	Expect(couldNotStartCount).To(Equal(expectedCouldNotStart), "couldNotStart")
	Expect(failedInstances).To(ConsistOf(expectedFailedInstances), "failedInstances")
}

func hasReportedAttempts(fakeListener *fakes.FakeListener, count, limit int) {
	Expect(fakeListener.RetryAttemptCallCount()).To(Equal(count))
	for i := 0; i < count; i++ {
		c, l := fakeListener.RetryAttemptArgsForCall(i)
		Expect(c).To(Equal(i + 1))
		Expect(l).To(Equal(limit))
	}
}

func hasReportedCanariesStarting(fakeListener *fakes.FakeListener, count int) {
	Expect(fakeListener.CanariesStartingCallCount()).To(Equal(count), "CanariesStarting() call count")
}

func hasReportedCanariesFinished(fakeListener *fakes.FakeListener, count int) {
	Expect(fakeListener.CanariesFinishedCallCount()).To(Equal(count), "CanariesFinished() call count")
}

func expectToHaveStarted(controllers ...*processController) {
	for _, c := range controllers {
		c.HasStarted()
	}
}

func expectToHaveNotStarted(controllers ...*processController) {
	for _, c := range controllers {
		c.DoesNotStart()
	}
}

func allowToProceed(controllers ...*processController) {
	for _, c := range controllers {
		c.AllowToProceed()
	}
}

type processController struct {
	name         string
	startedState bool
	started      chan bool
	canProceed   chan bool
}

func newProcessController(name string) *processController {
	return &processController{
		started:    make(chan bool, 1),
		canProceed: make(chan bool, 1),
		name:       name,
	}
}

func (p *processController) NotifyStart() {
	p.started <- true
}

func (p *processController) WaitForSignalToProceed() {
	<-p.canProceed
}

func (p *processController) HasStarted() {
	Eventually(p.started).Should(Receive(), fmt.Sprintf("Process %s expected to be in a started state", p.name))
}

func (p *processController) DoesNotStart() {
	Consistently(p.started).ShouldNot(Receive(), fmt.Sprintf("Process %s expected to be in a non-started state", p.name))
}

func (p *processController) AllowToProceed() {
	p.canProceed <- true
}
