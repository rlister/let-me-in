package main

import (
	"flag"
	"fmt"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/service/ec2"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"text/tabwriter"
)

var VERSION = "dev"

type LmiInput struct {
	GroupId    *string
	IpProtocol *string
	FromPort   *int64
	ToPort     *int64
	CidrIp     *string
}

// error handler
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}

// return array of security group objects for given group names
func getGroups(client *ec2.EC2, names []string) ([]*ec2.SecurityGroup, error) {

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
	if err != nil {
		return nil, err
	}

	return resp.SecurityGroups, nil
}

func authorizeGroup(client *ec2.EC2, group *ec2.SecurityGroup, input LmiInput) {
	_, err := client.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    group.GroupId,
		IpProtocol: input.IpProtocol,
		FromPort:   input.FromPort,
		ToPort:     input.ToPort,
		CidrIp:     input.CidrIp,
	})

	// be idempotent, i.e. skip error if this permission already exists in group
	if err != nil {
		if err.(awserr.Error).Code() != "InvalidPermission.Duplicate" {
			panic(err)
		}
	}
}

func revokeGroup(client *ec2.EC2, group *ec2.SecurityGroup, input LmiInput) {
	_, err := client.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:    group.GroupId,
		IpProtocol: input.IpProtocol,
		FromPort:   input.FromPort,
		ToPort:     input.ToPort,
		CidrIp:     input.CidrIp,
	})

	// be idempotent, i.e. skip error if this permission already exists in group
	if err != nil {
		if err.(awserr.Error).Code() != "InvalidPermission.NotFound" {
			panic(err)
		}
	}
}

// revoke all existing permissions for security group
func cleanGroup(client *ec2.EC2, group *ec2.SecurityGroup) {
	for _, perm := range group.IpPermissions {
		for _, cidr := range perm.IpRanges {
			revokeGroup(client, group, LmiInput{
				IpProtocol: perm.IpProtocol,
				FromPort:   perm.FromPort,
				ToPort:     perm.ToPort,
				CidrIp:     cidr.CidrIp,
			})
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

// parse cmdline argv into args before '--' and cmd to exec afterwards
func parseArgs(argv []string) ([]string, []string) {

	// if -- found, return slices before and after
	for i, arg := range flag.Args() {
		if arg == "--" {
			return argv[0:i], argv[i+1:]
		}
	}

	// no -- found, return all of argv
	return argv, nil
}

// print out table of current IP permissions for given security groups
func printIpRanges(groups []*ec2.SecurityGroup) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for _, group := range groups {
		for _, perm := range group.IpPermissions {
			for _, cidr := range perm.IpRanges {
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", *group.GroupName, *perm.IpProtocol, *cidr.CidrIp, *perm.FromPort)
			}
		}
	}
	w.Flush()
}

func main() {

	// cmdline options
	versionFlag := flag.Bool("v", false, "show version and exit")
	cidr := flag.String("c", "", "set a specific cidr block (default: current public ip)")
	protocol := flag.String("P", "TCP", "protocol to allow (default: TCP)")
	port := flag.Int("p", 22, "port number to allow (default: 22)")
	revoke := flag.Bool("r", false, "revoke access from security groups (default: false)")
	cleanFlag := flag.Bool("clean", false, "clean listed groups, i.e. revoke all access")
	list := flag.Bool("l", false, "list current rules for groups")
	flag.Parse()

	// show version and exit
	if *versionFlag {
		fmt.Printf("let-me-in %v\n", VERSION)
		return
	}

	// if cidr not given get ip from external service
	if *cidr == "" {
		ident := os.Getenv("LMI_IDENT_URL")
		if ident == "" {
			ident = "http://v4.ident.me/"
		}
		ip := getMyIp(ident) + "/32"
		cidr = &ip
	}

	input := LmiInput{
		IpProtocol: protocol,
		FromPort:   aws.Int64(int64(*port)),
		ToPort:     aws.Int64(int64(*port)),
		CidrIp:     cidr,
	}

	// configure aws-sdk from AWS_* env vars
	client := ec2.New(&aws.Config{})

	// get security group names and any command to exec after '--'
	groupNames, cmd := parseArgs(flag.Args())

	// get details for listed groups
	groups, err := getGroups(client, groupNames)
	if err != nil {
		fmt.Printf("%v\n", err) // if AWS creds not configured, report it here
		return
	}

	// print list of current IP permissions for groups
	if *list {
		printIpRanges(groups)
		return
	}

	if *cleanFlag {
		for _, group := range groups {
			cleanGroup(client, group)
		}
		return
	}

	// revoke on -r option
	if *revoke {
		for _, group := range groups {
			revokeGroup(client, group, input)
		}
		return
	}

	// default behaviour
	for _, group := range groups {
		authorizeGroup(client, group, input)
	}

	// exec any command after '--', then revoke
	if cmd != nil {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Stdout = os.Stdout
		c.Stdin = os.Stdin
		c.Stderr = os.Stderr

		err := c.Run()
		if err != nil {
			fmt.Println(err) // show err and keep running so we hit revoke below
		}

		for _, group := range groups {
			revokeGroup(client, group, input)
		}
	}

}
