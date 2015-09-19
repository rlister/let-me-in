# let-me-in

Here's the deal: you want to ssh to some AWS instances. Not often,
obviously, because that's lame and you have much better ways of
provisioning and monitoring than that. You're no chump.

But let's face it, sometimes the shit hits the fan, and we all know
it's guaranteed to be at 3am, and you just _need_ to get into those hosts
and fix stuff.

You don't want to leave port 22 open to the internet in your security
groups, and you never know what your external IP will be. `let-me-in`
takes care of that problem, by temporarily adding your IP (or CIDR
block) to a security group for the duration of your hour of need.

## Installation

### Homebrew

You can add `rlister/let-me-in` as a tap and install using homebrew:

```
brew tap rlister/let-me-in
brew install let-me-in
```

### Binaries for linux and OSX

Download a static binary from
https://github.com/rlister/let-me-in/releases.

### Docker

```
docker pull rlister/let-me-in:latest
docker run rlister/let-me-in -v
```

Unless running under an IAM role, you will need to pass in your AWS
credentials to make AWS changes, for example:

```
docker run \
  -e AWS_REGION \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  rlister/let-me-in my-security-group
```

### Build from source

Build your own using your favourite `go build` command, for example:

```
go build -ldflags "-X main.VERSION=x.y.z" ./let-me-in.go
```

## Configuration

You will need to set your AWS credential in the environment:

```
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID='xxx'
export AWS_SECRET_ACCESS_KEY='xxx'
```

## IAM permissions example

In order to modify a security group, you will need to add an IAM
policy something like this. Use multiple ARNs or wildcards for all
security groups that require access for `let-me-in`.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:AuthorizeSecurityGroupIngress",
        "ec2:RevokeSecurityGroupIngress"
      ],
      "Resource": [
        "arn:aws:ec2:us-east-1:*:security-group/sg-xxx"
      ]
    }
  ]
}
```

## Usage

Note: all commands below may be given with multiple security groups on
the command-line.

By default `let-me-in` will look up your external IP address at
http://v4.ident.me/, and add the address to the named security group(s):

```
let-me-in my-security-group
```

Skip the lookup and specify any CIDR block using `-cidr` option:

```
let-me-in -cidr 1.2.3.4/32 my-security-group
```

Default port allowed is `22`, but you can, for example, open a
webserver for testing using:

```
let-me-in -port 80 my-security-group
```

Once done, don't forget to revoke the security group entry:

```
let-me-in -revoke my-security-group
```

or

```
let-me-in -r my-security-group
```

List the current permissions for security groups with:

```
let-me-in -list my-security-group
```

or

```
let-me-in -l my-security-group
```

## Implicit commands

When access is needed for just a single command, you may run the
verbose:

```
let-me-in my-sg
ssh my-host.example.com
let-me-in -r my-sg
```

or, alternatively, you may embed a command to run after the argument
`--`:

```
let-me-in my-sg -- ssh my-host.example.com
```

In this case, `let-me-in` will authorize access, run the ssh
command, and, when it exits, revoke access again.

## Cleanup

If you wish to remove *all* permissions for a group:

```
let-me-in -clean my-security-group
```

Be careful, your security group will have no ingress at all after this
command.

## Bugs

Should probably trap signals in implicit commands and ensure revoke
gets run before exit.

## Packaging

This is how I build binaries, packagers or forkers may follow
something similar.

### Dependencies

Dependencies are vendored using
[godep](https://github.com/tools/godep). Install `godep` with:

```
go get github.com/tools/godep
```

then install or update any deps locally with:

```
go get -u github.com/foo/bar
godep save -r
```

### Making a new release

Binaries in releases are built using
[goxc](https://github.com/laher/goxc):

```
goxc -t    # first use
goxc bump
goxc -bc="linux darwin"
```

## Docker image

I build a tiny docker image from scratch as follows:

```
version=0.1.0
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags "-X main.VERSION=${version}" let-me-in.go
docker build -t rlister/let-me-in:${version} .
docker tag -f rlister/let-me-in:${version} rlister/let-me-in:latest
docker push rlister/let-me-in
```

## License

[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)
