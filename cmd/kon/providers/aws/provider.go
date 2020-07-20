package aws

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"

	"github.com/k11n/konstellation/cmd/kon/config"
	"github.com/k11n/konstellation/cmd/kon/kube"
	"github.com/k11n/konstellation/cmd/kon/utils"
	kaws "github.com/k11n/konstellation/pkg/cloud/aws"
	"github.com/k11n/konstellation/pkg/components"
	"github.com/k11n/konstellation/pkg/components/ingress"
	"github.com/k11n/konstellation/pkg/utils/async"
)

type AWSProvider struct {
	id string
}

func NewAWSProvider() *AWSProvider {
	provider := AWSProvider{
		id: "aws",
	}

	return &provider
}

func (a *AWSProvider) ID() string {
	return a.id
}

func (a *AWSProvider) String() string {
	return "AWS"
}

func (a *AWSProvider) IsSetup() bool {
	return config.GetConfig().Clouds.AWS.IsSetup()
}

func (a *AWSProvider) Setup() error {
	conf := config.GetConfig()
	awsConf := &conf.Clouds.AWS
	creds := awsConf.GetDefaultCredentials()

	prompt := promptui.Prompt{
		Label:     "AWS Access Key ID",
		Default:   creds.AccessKeyID,
		AllowEdit: true,
		Validate:  utils.ValidateMinLength(10),
	}
	utils.FixPromptBell(&prompt)

	val, err := prompt.Run()
	if err != nil {
		return err
	}
	creds.AccessKeyID = val

	prompt.Label = "AWS Secret Access Key"
	prompt.Default = creds.SecretAccessKey
	val, err = prompt.Run()
	if err != nil {
		return err
	}
	creds.SecretAccessKey = val
	awsConf.Credentials = creds

	// prompt for regions
	regions := awsConf.Regions
	if len(regions) == 0 {
		regions = []string{"us-east-1", "us-west-2"}
	}
	validRegions := map[string]bool{}
	resolver := endpoints.DefaultResolver()
	partitions := resolver.(endpoints.EnumPartitions).Partitions()
	for _, p := range partitions {
		if p.ID() == "aws" {
			for id := range p.Regions() {
				validRegions[id] = true
			}
		}
	}
	regionPrompt := promptui.Prompt{
		Label:   "Regions to use (separate multiple regions with comma)",
		Default: strings.Join(regions, ","),
		Validate: func(s string) error {
			regions := strings.Split(s, ",")
			for _, r := range regions {
				r = strings.TrimSpace(r)
				if !validRegions[r] {
					return fmt.Errorf("Invalid region: %s", r)
				}
			}
			if len(regions) == 0 {
				return fmt.Errorf("no regions provided")
			}
			return nil
		},
	}
	utils.FixPromptBell(&regionPrompt)

	res, err := regionPrompt.Run()
	if err != nil {
		return err
	}

	// validate against actual regions
	awsConf.Regions = []string{}
	for _, r := range strings.Split(res, ",") {
		r = strings.TrimSpace(r)
		if len(r) == 0 {
			continue
		}
		awsConf.Regions = append(awsConf.Regions, r)
	}

	// check if key works
	sess, err := sessionForRegion(awsConf.Regions[0])
	if err != nil {
		return errors.Wrapf(err, "AWS credentials are not valid")
	}

	iamSvc := iam.New(sess)
	user, err := iamSvc.GetUser(&iam.GetUserInput{})
	if err != nil {
		return errors.Wrapf(err, "Couldn't make authenticated calls using provided credentials")
	}

	resp, err := iamSvc.SimulatePrincipalPolicy(&iam.SimulatePrincipalPolicyInput{
		ActionNames: []*string{
			// State bucket
			aws.String("s3:GetObject"),
			aws.String("s3:CreateBucket"),
			aws.String("s3:ListBucket"),
			aws.String("s3:GetBucketLocation"),
			// Cluster info
			aws.String("eks:ListClusters"),
			aws.String("eks:DescribeCluster"),
			aws.String("eks:DescribeNodegroup"),
		},
		PolicySourceArn: user.User.Arn,
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to check AWS permissions")
	}
	for _, res := range resp.EvaluationResults {
		if *res.EvalDecision != iam.PolicyEvaluationDecisionTypeAllowed {
			return fmt.Errorf("missing %s permission", *res.EvalActionName)
		}
	}

	// ask for a bucket to store state
	awsConf.StateS3Bucket, awsConf.StateS3BucketRegion, err = a.createStateBucket(sess, awsConf.StateS3Bucket)
	if err != nil {
		return err
	}

	fmt.Println("AWS has been configured to use with Konstellation!")
	fmt.Println("next, try creating a cluster with `kon cluster create`")
	return conf.Persist()
}

func (a *AWSProvider) GetComponents() []components.ComponentInstaller {
	comps := make([]components.ComponentInstaller, 0, len(kube.KubeComponents)+1)
	comps = append(comps, kube.KubeComponents...)
	comps = append(comps, &ingress.AWSALBIngress{})
	return comps
}

func (a *AWSProvider) createStateBucket(sess *session.Session, defaultBucket string) (bucketName string, bucketRegion string, err error) {
	s3Svc := s3.New(sess)

	listRes, err := s3Svc.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return
	}

	wp := async.NewWorkerPool()
	for i := range listRes.Buckets {
		bucket := listRes.Buckets[i]
		wp.AddTask(func() (interface{}, error) {
			bi, err := getBucketInfo(*bucket.Name, sess)
			if err != nil {
				return false, err
			}

			regionS3 := s3.New(sess, &aws.Config{
				Region: &bi.region,
			})
			res, err := regionS3.GetBucketTagging(&s3.GetBucketTaggingInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				// ignore tag errors, when there are no tags, it returns NoSuchTagSetError
				return false, nil
			}

			for _, tag := range res.TagSet {
				if *tag.Key == kaws.TagKonstellation {
					return true, nil
				}
			}
			return false, nil
		})
	}
	wp.StopWait()

	konBuckets := make(map[string]bool)
	bucketNames := make([]string, 0)
	for i, task := range wp.GetTasks() {
		bucket := listRes.Buckets[i]
		// return error
		if task.Err != nil {
			err = errors.Wrapf(task.Err, "could not get bucket info: %s", *bucket.Name)
			return
		}

		if res, ok := task.Result.(bool); ok && res {
			konBuckets[*bucket.Name] = res
			bucketNames = append(bucketNames, *bucket.Name)
		}
	}

	// sort, ensure kon buckets appear first
	sort.Slice(bucketNames, func(i, j int) bool {
		if konBuckets[bucketNames[i]] == konBuckets[bucketNames[j]] {
			return strings.Compare(bucketNames[i], bucketNames[j]) < 0
		}
		if konBuckets[bucketNames[i]] {
			return true
		}
		return false
	})

	fmt.Println("Konstellation needs to store configuration in a S3 bucket, select an existing one or create new.")
	fmt.Println("WARNING: if you are trying to manage existing Konstellation clusters, select the same state bucket.")

	// select existing bucket
	bucketNames = append(bucketNames, "Create new")
	bucketSelect := utils.NewPromptSelect("Select state bucket", bucketNames)
	idx, bucketName, err := bucketSelect.Run()
	if err != nil {
		return
	}

	// selected existing bucket
	if idx != len(bucketNames)-1 {
		bi, err := getBucketInfo(bucketName, sess)
		if err != nil {
			return "", "", err
		}
		return bucketName, bi.region, nil
	}

	bucketPromptFunc := func() (string, error) {
		// create new bucket
		bucketPrompt := promptui.Prompt{
			Label:   "Bucket name",
			Default: defaultBucket,
		}
		utils.FixPromptBell(&bucketPrompt)
		bn, err := bucketPrompt.Run()
		if err != nil {
			return "", err
		}
		bi, err := getBucketInfo(bn, sess)
		if err != nil {
			return "", err
		}
		if !bi.exists || bi.hasPermission {
			// we can use it
			return bn, nil
		} else {
			return "", fmt.Errorf("bucket already exists")
		}
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		os.Exit(1)
	}()

	for {
		bucketName, err = bucketPromptFunc()
		if err != nil {
			if err != promptui.ErrAbort && err != promptui.ErrInterrupt {
				fmt.Println("Bucket name already in use, please try another name")
			} else {
				// aborted or interrupted
				return
			}
			continue
		}
		break
	}

	bi, err := getBucketInfo(bucketName, sess)
	if err != nil {
		return
	}

	bucketRegion = bi.region

	if !bi.exists {
		// bucket doesn't exist, try to create it
		bucketPrompt := promptui.Prompt{
			Label:     fmt.Sprintf("Bucket %s doesn't exist, ok to create?", bucketName),
			IsConfirm: true,
			Default:   "Y",
		}
		utils.FixPromptBell(&bucketPrompt)
		if _, err = bucketPrompt.Run(); err != nil {
			return
		}

		bucketRegion = *s3Svc.Config.Region
		_, err = s3Svc.CreateBucket(&s3.CreateBucketInput{
			Bucket: &bucketName,
		})
		if err != nil {
			err = errors.Wrap(err, "Could not create bucket")
			return
		}

		// tag selected bucket
		_, err = s3Svc.PutBucketTagging(&s3.PutBucketTaggingInput{
			Bucket: &bucketName,
			Tagging: &s3.Tagging{
				TagSet: []*s3.Tag{
					{
						Key:   aws.String(kaws.TagKonstellation),
						Value: aws.String(kaws.TagValue1),
					},
				},
			},
		})
	}

	return
}

type bucketInfo struct {
	name          string
	region        string
	hasPermission bool
	exists        bool
}

// TODO: functional test for this
func getBucketInfo(bucket string, session *session.Session) (*bucketInfo, error) {
	s3Svc := s3.New(session)
	_, err := s3Svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: &bucket,
	})
	bi := &bucketInfo{}
	bi.name = bucket
	bi.region = *s3Svc.Config.Region
	if err == nil {
		// exists and user owns it
		bi.hasPermission = true
		bi.exists = true
		return bi, nil
	}
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == "NotFound" {
			// new bucket, we'll create it
			bi.hasPermission = true
			bi.exists = false
			return bi, nil
		}
	} else {
		return nil, err
	}

	// at this point, we got an AWS error, which either means we don't have perms, or it's not in the right region
	// see if we can get region info
	res, err := s3Svc.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: &bucket})
	if err != nil {
		// when we receive an AWS error for this, that likely means it belongs to someone else.
		if _, ok := err.(awserr.Error); ok {
			bi.hasPermission = false
			bi.exists = true
			return bi, nil
		}
		return nil, err
	}

	// not in the current location
	if res.LocationConstraint == nil {
		// legacy: us-east-1 has a constraint of null
		bi.region = "us-east-1"
	} else {
		bi.region = *res.LocationConstraint
	}
	bi.exists = true
	bi.hasPermission = true

	return bi, nil
}
