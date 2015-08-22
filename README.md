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

## Usage

## SSH configuration
