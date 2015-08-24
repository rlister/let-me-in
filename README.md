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

Download a static binary from
https://github.com/rlister/let-me-in/releases,
or build your own using your favourite `go build` command.

Binaries in releases are built using
[goxc](https://github.com/laher/goxc):

```
goxc -t    # first use
goxc bump
goxc -bc="linux darwin"
```

## Usage

By default `let-me-in` will look up your external IP address at
http://ident.me/, and add the address to the named security group(s):

```
let-me-in my-security-group
```

Skip the lookup and specify any CIDR block using `-c` option:

```
let-me-in -c 1.2.3.4/32 my-security-group
```

Default port allowed is `22`, but you can, for example, open a
webserver for testing using:

```
let-me-in -p 80 my-security-group
```

Once done, don't forget to revoke the security group entry:

```
let-me-in -r my-security-group
```

## SSH configuration
