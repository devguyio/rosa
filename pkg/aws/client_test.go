package aws_test

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/openshift-online/ocm-sdk-go/helpers"

	awsSdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	common "github.com/openshift-online/ocm-common/pkg/aws/validations"
	"github.com/openshift/rosa/pkg/aws"
	"github.com/openshift/rosa/pkg/aws/mocks"
	rosaTags "github.com/openshift/rosa/pkg/aws/tags"
)

var _ = Describe("Client", func() {
	var (
		client   aws.Client
		mockCtrl *gomock.Controller

		mockEC2API            *mocks.MockEc2ApiClient
		mockCfAPI             *mocks.MockCloudFormationApiClient
		mockIamAPI            *mocks.MockIamApiClient
		mockS3API             *mocks.MockS3ApiClient
		mockSecretsManagerAPI *mocks.MockSecretsManagerApiClient
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockCfAPI = mocks.NewMockCloudFormationApiClient(mockCtrl)
		mockIamAPI = mocks.NewMockIamApiClient(mockCtrl)
		mockEC2API = mocks.NewMockEc2ApiClient(mockCtrl)
		mockS3API = mocks.NewMockS3ApiClient(mockCtrl)
		mockSecretsManagerAPI = mocks.NewMockSecretsManagerApiClient(mockCtrl)
		client = aws.New(
			logrus.New(),
			mockIamAPI,
			mockEC2API,
			mocks.NewMockOrganizationsApiClient(mockCtrl),
			mockS3API,
			mockSecretsManagerAPI,
			mocks.NewMockStsApiClient(mockCtrl),
			mockCfAPI,
			mocks.NewMockServiceQuotasApiClient(mockCtrl),
			&aws.AccessKey{},
			false,
		)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("EnsureOsdCcsAdminUser", func() {
		var (
			stackName     string
			stackStatus   string
			adminUserName string
		)
		BeforeEach(func() {
			stackName = "fake-stack"
			adminUserName = "fake-admin-username"
		})
		Context("When the cloudformation stack already exists", func() {
			JustBeforeEach(func() {
				mockCfAPI.EXPECT().ListStacks(context.Background(), &cloudformation.ListStacksInput{}).Return(
				&cloudformation.ListStacksOutput{
					StackSummaries: []cloudformationtypes.StackSummary{
						{
							StackName:   &stackName,
							StackStatus: cloudformationtypes.StackStatus(stackStatus),
						},
					},
				}, nil)
			})

			Context("When stack is in CREATE_COMPLETE state", func() {
				BeforeEach(func() {
					stackStatus = string(cloudformationtypes.StackStatusCreateComplete)
					mockIamAPI.EXPECT().GetUser(context.Background(), &iam.GetUserInput{UserName: &adminUserName}).Return(
						&iam.GetUserOutput{User: &iamtypes.User{UserName: &adminUserName}},
						&iamtypes.NoSuchEntityException{},
					)
					mockCfAPI.EXPECT().UpdateStack(context.Background(), gomock.Any()).Return(nil, nil)
				})
				It("Returns without error", func() {
					err := IsStackUpdateComplete(stackName, mockCfAPI)
					Expect(err).NotTo(HaveOccurred())

					stackCreated, err := client.EnsureOsdCcsAdminUser(stackName, adminUserName, aws.DefaultRegion)

					Expect(stackCreated).To(BeFalse())
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("When stack is in DELETE_COMPLETE state", func() {
				BeforeEach(func() {
					stackStatus = string(cloudformationtypes.StackStatusDeleteComplete)
					mockIamAPI.EXPECT().ListUsers(context.Background(), gomock.Any()).Return(&iam.ListUsersOutput{Users: []iamtypes.User{}}, nil)
					mockIamAPI.EXPECT().TagUser(context.Background(), gomock.Any()).Return(&iam.TagUserOutput{}, nil)
					mockIamAPI.EXPECT().GetUser(context.Background(), &iam.GetUserInput{UserName: &adminUserName}).Return(
						&iam.GetUserOutput{User: &iamtypes.User{UserName: &adminUserName}},
						&iamtypes.NoSuchEntityException{},
					)
					mockCfAPI.EXPECT().CreateStack(context.Background(), gomock.Any()).Return(nil, nil)
				})
				It("Creates a cloudformation stack", func() {
					err := IsStackUpdateComplete(stackName, mockCfAPI)
					Expect(err).NotTo(HaveOccurred())

					stackCreated, err := client.EnsureOsdCcsAdminUser(stackName, adminUserName, aws.DefaultRegion)

					Expect(stackCreated).To(BeTrue())
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("When stack is in ROLLBACK_COMPLETE state", func() {
				BeforeEach(func() {
					stackStatus = string(cloudformationtypes.StackStatusRollbackComplete)
					mockIamAPI.EXPECT().GetUser(context.Background(), gomock.Any()).Return(
						&iam.GetUserOutput{User: &iamtypes.User{UserName: &adminUserName}},
						&iamtypes.NoSuchEntityException{},
					)
				})

				It("Returns error telling the stack is in an invalid state", func() {
					stackCreated, err := client.EnsureOsdCcsAdminUser(stackName, adminUserName, aws.DefaultRegion)

					Expect(stackCreated).To(BeFalse())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(
						"exists with status ROLLBACK_COMPLETE. Expected status is CREATE_COMPLETE"))
				})
			})
		})

		Context("When the cloudformation stack does not exists", func() {
			BeforeEach(func() {
				mockCfAPI.EXPECT().ListStacks(context.Background(), gomock.Any()).Return(&cloudformation.ListStacksOutput{
					StackSummaries: []cloudformationtypes.StackSummary{},
				}, nil)
				mockIamAPI.EXPECT().ListUsers(context.Background(), gomock.Any()).Return(&iam.ListUsersOutput{Users: []iamtypes.User{}}, nil)
				mockIamAPI.EXPECT().TagUser(context.Background(), gomock.Any()).Return(&iam.TagUserOutput{}, nil)
				mockIamAPI.EXPECT().GetUser(context.Background(), gomock.Any()).Return(
					&iam.GetUserOutput{User: &iamtypes.User{UserName: &adminUserName}},
					&iamtypes.NoSuchEntityException{},
				)
				mockCfAPI.EXPECT().CreateStack(context.Background(), gomock.Any()).Return(nil, nil)
			})

			It("Creates a cloudformation stack", func() {
				err := IsStackUpdateComplete(stackName, mockCfAPI)
				Expect(err).NotTo(HaveOccurred())

				stackCreated, err := client.EnsureOsdCcsAdminUser(stackName, adminUserName, aws.DefaultRegion)

				Expect(err).NotTo(HaveOccurred())
				Expect(stackCreated).To(BeTrue())
			})
		})
		//		Context("When the IAM user already exists"), func() {
		//			BeforeEach(func() {

		//			}
	})
	Context("CheckAdminUserNotExisting", func() {
		var (
			adminUserName string
		)
		BeforeEach(func() {
			adminUserName = "fake-admin-username"
			mockIamAPI.EXPECT().ListUsers(context.Background(), gomock.Any()).Return(&iam.ListUsersOutput{
				Users: []iamtypes.User{
					{
						UserName: &adminUserName,
					},
				},
			}, nil)
		})
		Context("When admin user already exists", func() {
			It("returns an error", func() {
				err := client.CheckAdminUserNotExisting(adminUserName)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Error creating user: IAM user"))
			})
		})
		Context("When admin user does not exist", func() {
			var (
				secondFakeAdminUserName string
			)
			BeforeEach(func() {
				secondFakeAdminUserName = "second-fake-admin-username"
			})
			It("returns true", func() {
				err := client.CheckAdminUserNotExisting(secondFakeAdminUserName)

				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Context("Get Account Role By ARN", func() {

		var testArn = "arn:aws:iam::765374464689:role/test-Installer-Role"
		var testName = "test-Installer-Role"
		var tags = []iamtypes.Tag{
			{Key: awsSdk.String(common.ManagedPolicies), Value: awsSdk.String(rosaTags.True)},
			{Key: awsSdk.String(rosaTags.RoleType), Value: awsSdk.String(aws.InstallerAccountRole)},
		}

		It("Finds and Returns Account Role", func() {

			mockIamAPI.EXPECT().GetRole(context.Background(), gomock.Any()).Return(&iam.GetRoleOutput{
				Role: &iamtypes.Role{
					Arn:      &testArn,
					RoleName: &testName,
				},
			}, nil)

			mockIamAPI.EXPECT().ListRoleTags(context.Background(), gomock.Any()).Return(&iam.ListRoleTagsOutput{
				Tags: tags,
			}, nil)

			mockIamAPI.EXPECT().ListRolePolicies(context.Background(),gomock.Any()).Return(&iam.ListRolePoliciesOutput{
				PolicyNames: make([]string, 0),
			}, nil)

			role, err := client.GetAccountRoleByArn(testArn)

			Expect(err).NotTo(HaveOccurred())
			Expect(role).NotTo(BeNil())

			Expect(role.RoleName).To(Equal(testName))
			Expect(role.RoleARN).To(Equal(testArn))
			Expect(role.RoleType).To(Equal(aws.InstallerAccountRoleType))
		})

		It("Returns nil when No Role with ARN exists", func() {
			mockIamAPI.EXPECT().GetRole(context.Background(), gomock.Any()).Return(nil, fmt.Errorf("role Doesn't Exist"))

			role, err := client.GetAccountRoleByArn(testArn)

			Expect(role).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("Returns nil when the Role exists, but it is not an Account Role", func() {

			var roleName = "not-an-account-role"

			mockIamAPI.EXPECT().GetRole(context.Background(), gomock.Any()).Return(&iam.GetRoleOutput{
				Role: &iamtypes.Role{
					Arn:      &testArn,
					RoleName: &roleName,
				},
			}, nil)

			role, err := client.GetAccountRoleByArn(testArn)
			Expect(role).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("List Subnets", func() {

		subnetOneId := "test-subnet-1"
		subnetTwoId := "test-subnet-2"
		subnet := ec2types.Subnet{
			SubnetId: helpers.NewString(subnetOneId),
		}

		subnet2 := ec2types.Subnet{
			SubnetId: helpers.NewString(subnetTwoId),
		}

		var subnets []ec2types.Subnet
		subnets = append(subnets, subnet, subnet2)

		It("Lists all", func() {

			var request *ec2.DescribeSubnetsInput

			mockEC2API.EXPECT().DescribeSubnets(context.Background(), gomock.Any()).DoAndReturn(
				func(arg *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
					request = arg
					return &ec2.DescribeSubnetsOutput{
						Subnets: subnets,
					}, nil
				})

			subs, err := client.ListSubnets()
			Expect(err).NotTo(HaveOccurred())

			Expect(subs).To(HaveLen(2))
			Expect(request.SubnetIds).To(BeEmpty())
		})

		It("Lists by subnet ids", func() {

			var request *ec2.DescribeSubnetsInput

			mockEC2API.EXPECT().DescribeSubnets(context.Background(), gomock.Any()).DoAndReturn(
				func(arg *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
					request = arg
					return &ec2.DescribeSubnetsOutput{
						Subnets: subnets,
					}, nil
				})

			subs, err := client.ListSubnets(subnetOneId, subnetTwoId)
			Expect(err).NotTo(HaveOccurred())

			Expect(subs).To(HaveLen(2))
			Expect(request.SubnetIds).To(ContainElements(&subnetOneId, &subnetTwoId))

		})
	})
})


