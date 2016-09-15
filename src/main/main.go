package main

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/satori/go.uuid"
	"os"
	"log"
	"flag"
)

var awsRegion string = "eu-west-1"
var test bool = false

func main() {

	testVarPtr := flag.Bool("test", false, "Do a test run")
	flag.Parse()

	if *testVarPtr == true {
		test = true
	}

	var createSubDomainBucket bool = false

	var websiteName string = ""
	var withWww string = ""

	for {
		fmt.Printf("Enter a name for the website: ")
		fmt.Scanf("%s\n", &websiteName)

		if len(websiteName) > 0 {
			break
		}
	}

	fmt.Printf("I will create a bucket for %s\n", websiteName)

	for {
		fmt.Printf("Shall i also create a bucket for www.%s? [Y/n] ", websiteName)
		fmt.Scanf("%s\n", &withWww)

		if len(withWww) > 0 {
			break
		}
	}

	if withWww == "Y" || withWww == "y" {
		createSubDomainBucket = true
	}

	// Confirm
	fmt.Printf("I will create an AWS-user and S3-bucket. Is this OK? [Y/n] ")

	var okResult string = ""
	fmt.Scanf("%s\n", &okResult)

	if okResult != "Y" && okResult != "y" {
		fmt.Println("Exiting..")
		os.Exit(1)
	}

	// Start
	var channel chan string = make(chan string)
	var done chan bool = make(chan bool)
	var numberOfRoutines int = 2

	go func() {
		// Create user
		createAwsUser(websiteName, channel)
		done <- true
	}()

	go func() {
		// Create bucket
		createAwsBucket(websiteName, channel)
		done <- true
	}()

	if createSubDomainBucket == true {
		numberOfRoutines++
		go func() {
			// Create bucket
			createAwsBucket(fmt.Sprintf("www.%s", websiteName), channel)
			done <- true
		}()
	}

	go func() {
		for s := range channel {
			fmt.Println(s)
		}
	}()

	for i := 0; i < numberOfRoutines; i++ {
		<-done
	}

	fmt.Print("\nAll done.\n")
}

func createAwsUser(websiteName string, channel chan string) {

	iamClient := iam.New(session.New())

	/**
	 * User
	 */
	var userName string = fmt.Sprintf("surreal-%s", websiteName)
	userInput := &iam.CreateUserInput{
		UserName: &userName,
	}

	if test == false {
		userOutput, err := iamClient.CreateUser(userInput)
		if err != nil {
			log.Printf("%s\n", err)
			return
		}

		channel <- fmt.Sprintf("User created %s", *userOutput.User.UserId)
	} else {
		channel <- fmt.Sprintf("User created %s", userName)
	}

	/**
	 * Inline policy
	 */
	var sID uuid.UUID = uuid.NewV4()
	var policyName string = fmt.Sprintf("surreal-%s", websiteName)
	var policyDocument string = fmt.Sprintf(
		"{\"Version\": \"2012-10-17\",\"Statement\": [{\"Sid\": \"%s\",\"Effect\": \"Allow\",\"Action\": [ \"s3:*\"],\"Resource\": [\"arn:aws:s3:::%s\",\"arn:aws:s3:::%s/*\"]}]}",
		sID, websiteName, websiteName)

	putUserPolicyInput := &iam.PutUserPolicyInput{
		UserName:       &userName,
		PolicyName:     &policyName,
		PolicyDocument: &policyDocument,
	}

	// Create policy
	if test == false {
		_, err := iamClient.PutUserPolicy(putUserPolicyInput)
		if err != nil {
			log.Printf("%s\n", err)
			return
		}
	}

	channel <- "Inline policy created"

	if test == false {
		/**
		 * Access key
		 */
		accessKeyInput := &iam.CreateAccessKeyInput{
			UserName: &userName,
		}

		accessKeyOutput, err := iamClient.CreateAccessKey(accessKeyInput)
		if err != nil {
			log.Printf("%s\n", err)
			return
		}

		channel <- fmt.Sprintf("Access-key and Secret-key created!\nAccess key: %s\nSecret key: %s", *accessKeyOutput.AccessKey.AccessKeyId, *accessKeyOutput.AccessKey.SecretAccessKey)
	} else {
		channel <- "Access-key and Secret-key created!"
	}
}

func createAwsBucket(websiteName string, channel chan string) {

	/**
	 * Create bucket
	 */
	s3Client := s3.New(session.New(&aws.Config{
		Region: &awsRegion,
	}))

	var acl string = "public-read"
	createBucketInput := &s3.CreateBucketInput{
		ACL:    &acl,
		Bucket: &websiteName,
	}

	if test == false {
		createBucketOutput, err := s3Client.CreateBucket(createBucketInput)
		if err != nil {
			log.Printf("%s\n", err)
			return
		}

		channel <- fmt.Sprintf("Bucket \"%s\" created", *createBucketOutput.Location)
	} else {
		channel <- fmt.Sprintf("Bucket \"%s\" created", websiteName)
	}

	if test == false {
		/**
		* Enable static website S3
		 */
		var indexDocument string = "index.html"
		var errorDocument string = "error.html"
		putBucketWebsiteInput := &s3.PutBucketWebsiteInput{
			Bucket: &websiteName,
			WebsiteConfiguration: &s3.WebsiteConfiguration{
				IndexDocument: &s3.IndexDocument{
					Suffix: &indexDocument,
				},
				ErrorDocument: &s3.ErrorDocument{
					Key: &errorDocument,
				},
			},
		}
		_, err := s3Client.PutBucketWebsite(putBucketWebsiteInput)
		if err != nil {
			log.Printf("%s\n", err)
			return
		}
	}

	channel <- fmt.Sprintf("Bucket website-hosting turned on")
}
