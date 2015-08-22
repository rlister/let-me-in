package main

import (
	"flag"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"io/ioutil"
	"net/http"
	"os"
)

// error handler
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}

// convert group names to ids, which are needed for vpcs
func getGroupIds(client *ec2.EC2, names []string) []*string {

	// get names as array of aws.String objects
	values := make([]*string, len(names))
	for i, name := range names {
		values[i] = aws.String(name)
	}

	// request params as filter for names
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-name"),
				Values: values,
			},
		},
	}

	// send request
	resp, err := client.DescribeSecurityGroups(params)
	check(err)

	// return ids
	for i, group := range resp.SecurityGroups {
		values[i] = group.GroupId
	}

	return values
}

// authorize given security group
func authorizeGroup(client *ec2.EC2, id *string, protocol *string, port *int, cidr *string) {

	// make the request
	params := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(*id),
		IpProtocol: aws.String(*protocol),
		FromPort:   aws.Int64(int64(*port)),
		ToPort:     aws.Int64(int64(*port)),
		CidrIp:     aws.String(*cidr),
	}

	_, err := client.AuthorizeSecurityGroupIngress(params)

	// be idempotent, i.e. skip error if this permission already exists in group
	if err != nil {
		if err.(awserr.Error).Code() != "InvalidPermission.Duplicate" {
			panic(err)
		}
	}
}

// revoke permission from security group
func revokeGroup(client *ec2.EC2, id *string, protocol *string, port *int, cidr *string) {

	// make the request
	params := &ec2.RevokeSecurityGroupIngressInput{
		GroupId:    aws.String(*id),
		IpProtocol: aws.String(*protocol),
		FromPort:   aws.Int64(int64(*port)),
		ToPort:     aws.Int64(int64(*port)),
		CidrIp:     aws.String(*cidr),
	}

	_, err := client.RevokeSecurityGroupIngress(params)

	// be idempotent, i.e. skip error if this permission does not exist in group
	if err != nil {
		if err.(awserr.Error).Code() != "InvalidPermission.NotFound" {
			panic(err)
		}
	}
}

// get my external-facing IP as a string
func getMyIp(ident string) string {
	resp, err := http.Get(ident)
	check(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	check(err)

	return string(body)
}

func main() {

	// cmdline options
	cidr := flag.String("c", "", "set a specific cidr block (default: current public ip)")
	protocol := flag.String("P", "TCP", "protocol to allow (default: TCP)")
	port := flag.Int("p", 22, "port number to allow (default: 22)")
	revoke := flag.Bool("r", false, "revoke access from security groups (default: false)")
	flag.Parse()

	// if cidr not given get ip from external service
	if *cidr == "" {
		ident := os.Getenv("LMI_IDENT_URL")
		if ident == "" {
			ident = "http://ident.me/"
		}
		ip := getMyIp(ident) + "/32"
		cidr = &ip
	}

	// configure aws-sdk from AWS_* env vars
	client := ec2.New(&aws.Config{})

	// convert security group names to ids for vpc
	ids := getGroupIds(client, flag.Args())

	// action to take, default auth, or revoke on -r
	action := authorizeGroup
	if *revoke {
		action = revokeGroup
	}

	// call action for all the requested security groups
	for _, id := range ids {
		action(client, id, protocol, port, cidr)
	}
}
