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

// error handler
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}

// convert group names to ids, which are needed for vpcs
func getGroupIds(client *ec2.EC2, names []string) ([]*string, error) {

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

	// return ids
	for i, group := range resp.SecurityGroups {
		values[i] = group.GroupId
	}

	return values, nil
}

// call authorize for all the requested security groups
func authorizeGroups(client *ec2.EC2, ids []*string, protocol *string, port *int, cidr *string) {
	for _, id := range ids {
		authorizeGroup(client, id, protocol, port, cidr)
	}
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

// call revoke for all the requested security groups
func revokeGroups(client *ec2.EC2, ids []*string, protocol *string, port *int, cidr *string) {
	for _, id := range ids {
		revokeGroup(client, id, protocol, port, cidr)
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

	// configure aws-sdk from AWS_* env vars
	client := ec2.New(&aws.Config{})

	// get security group names and any command to exec after '--'
	groups, cmd := parseArgs(flag.Args())

	// convert security group names to ids for vpc
	ids, err := getGroupIds(client, groups)
	if err != nil {
		fmt.Printf("%v\n", err)
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
		revokeGroups(client, ids, protocol, port, cidr)
	} else {
		authorizeGroups(client, ids, protocol, port, cidr)

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
			revokeGroups(client, ids, protocol, port, cidr)
		}
	}

}
