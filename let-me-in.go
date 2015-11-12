package main

import (
	"fmt"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rlister/let-me-in/Godeps/_workspace/src/github.com/jessevdk/go-flags"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"text/tabwriter"
)

var VERSION = "dev"

var opt struct {
	Version  bool   `short:"v" long:"version" description:"show version and exit"`
	List     bool   `short:"l" long:"list" description:"list current rules for security groups"`
	Cidr     string `short:"c" long:"cidr" description:"set a specific cidr block (default: current public ip)"`
	Port     int    `short:"p" long:"port" default:"22" description:"port number to allow"`
	Protocol string `short:"P" long:"protocol" default:"tcp" description:"protocol to allow: tcp, udp or icmp"`
	Revoke   bool   `short:"r" long:"revoke" description:"revoke access from security groups"`
	Clean    bool   `short:"x" long:"clean" description:"clean listed groups, i.e. revoke all access"`
	Filter   string `short:"f" long:"filter" default:"group-name" description:"filter to use for groups"`
	Ident    string `long:"ident" default:"http://v4.ident.me/" env:"LMI_IDENT_URL" description:"URL for ident service"`
}

type Input struct {
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

// return array of security group objects for given groups, by filter (group-name, group-id, tag:Name, etc)
func getGroups(client *ec2.EC2, names []string, filter string) ([]*ec2.SecurityGroup, error) {

	// get names as array of aws.String objects
	values := make([]*string, len(names))
	for i, name := range names {
		values[i] = aws.String(name)
	}

	// request params as filter for names
	params := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(filter),
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

func authorizeGroups(client *ec2.EC2, groups []*ec2.SecurityGroup, input Input) {
	for _, group := range groups {
		authorizeGroup(client, group, input)
	}
}

// add given permission to security group
func authorizeGroup(client *ec2.EC2, group *ec2.SecurityGroup, input Input) {
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

func revokeGroups(client *ec2.EC2, groups []*ec2.SecurityGroup, input Input) {
	for _, group := range groups {
		revokeGroup(client, group, input)
	}
}

// revoke given permission for security group
func revokeGroup(client *ec2.EC2, group *ec2.SecurityGroup, input Input) {
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

func cleanGroups(client *ec2.EC2, groups []*ec2.SecurityGroup) {
	for _, group := range groups {
		cleanGroup(client, group)
	}
}

// revoke all existing permissions for security group
func cleanGroup(client *ec2.EC2, group *ec2.SecurityGroup) {
	for _, perm := range group.IpPermissions {
		for _, cidr := range perm.IpRanges {
			revokeGroup(client, group, Input{
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
func parseArgs(args []string) ([]string, []string) {

	// if -- found, return slices before and after
	for i, arg := range args {
		if arg == "--" {
			return args[0:i], args[i+1:]
		}
	}

	// no -- found, return all of args
	return args, nil
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
	flags.Parse(&opt)

	// show version and exit
	if opt.Version {
		fmt.Printf("let-me-in %v\n", VERSION)
		return
	}

	// if cidr not given get ip from external service
	if opt.Cidr == "" {
		opt.Cidr = getMyIp(opt.Ident) + "/32"
	}

	// requested permissions
	input := Input{
		IpProtocol: &opt.Protocol,
		FromPort:   aws.Int64(int64(opt.Port)),
		ToPort:     aws.Int64(int64(opt.Port)),
		CidrIp:     &opt.Cidr,
	}

	// get security group names and any command to exec after '--'
	groupNames, cmd := parseArgs(os.Args[1:])

	// configure aws-sdk from AWS_* env vars
	client := ec2.New(&aws.Config{})

	// get details for listed groups
	groups, err := getGroups(client, groupNames, opt.Filter)
	if err != nil {
		fmt.Printf("%v\n", err) // if AWS creds not configured, report it here
		return
	}

	// print list of current IP permissions for groups
	if opt.List {
		printIpRanges(groups)
		return
	}

	// remove all existing permissions for groups
	if opt.Clean {
		cleanGroups(client, groups)
		return
	}

	// revoke given permission for groups
	if opt.Revoke {
		revokeGroups(client, groups, input)
		return
	}

	// default behaviour
	authorizeGroups(client, groups, input)

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

		revokeGroups(client, groups, input)
	}
}
