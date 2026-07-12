# Contributing to ShadowLink

First off, thank you for considering contributing to ShadowLink! It's people like you that make ShadowLink such a great Decentralized VPN network.

## Where do I go from here?

If you've noticed a bug or have a feature request, make an issue! It's generally best if you get confirmation of your bug or approval for your feature request this way before starting to code.

## Fork & create a branch

If this is something you think you can fix, then fork ShadowLink and create a branch with a descriptive name.

A good branch name would be (where issue #325 is the ticket you're working on):

```sh
git checkout -b 325-add-udp-support
```

## Get the test suite running

Make sure you have **Go 1.21+** and **Flutter 3.10+** installed.

### Go Backend Daemon
Run the full test suite with race detection enabled before you start coding:
```sh
go test -race -v ./...
```

### Flutter GUI
Ensure the frontend is clean and tests pass:
```sh
cd shadowlink_gui
flutter analyze
flutter test
```

## Implement your fix or feature

At this point, you're ready to make your changes. Feel free to ask for help; everyone is a beginner at first. Make sure your code follows the standard Go formatting conventions (run `go fmt ./...`).

## Make a Pull Request

At this point, you should switch back to your master branch and make sure it's up to date with ShadowLink's master branch:

```sh
git remote add upstream git@github.com:TUSHAR91316/ShadowLink.git
git checkout master
git pull upstream master
```

Then update your feature branch from your local copy of master, and push it!

```sh
git checkout 325-add-udp-support
git rebase master
git push --set-upstream origin 325-add-udp-support
```

Finally, go to GitHub and make a Pull Request.

## Keeping your Pull Request updated

If a maintainer asks you to "rebase" your PR, they're saying that a lot of code has changed, and that you need to update your branch so it's easier to merge.
