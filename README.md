# Go Node Service SDK
[![Build Status](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/south-sdk-go/job/main/badge/icon)](https://jenkins.edgexfoundry.org/view/EdgeX%20Foundry%20Project/job/edgexfoundry/job/south-sdk-go/job/main/) [![Code Coverage](https://codecov.io/gh/edgexfoundry/south-sdk-go/branch/main/graph/badge.svg?token=NoUXyBZgt6)](https://codecov.io/gh/edgexfoundry/south-sdk-go) [![Go Report Card](https://goreportcard.com/badge/github.com/edgexfoundry/south-sdk-go)](https://goreportcard.com/report/github.com/edgexfoundry/south-sdk-go) [![GitHub Latest Dev Tag)](https://img.shields.io/github/v/tag/edgexfoundry/south-sdk-go?include_prereleases&sort=semver&label=latest-dev)](https://github.com/edgexfoundry/south-sdk-go/tags) ![GitHub Latest Stable Tag)](https://img.shields.io/github/v/tag/edgexfoundry/south-sdk-go?sort=semver&label=latest-stable) [![GitHub License](https://img.shields.io/github/license/edgexfoundry/south-sdk-go)](https://choosealicense.com/licenses/apache-2.0/) ![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/edgexfoundry/south-sdk-go) [![GitHub Pull Requests](https://img.shields.io/github/issues-pr-raw/edgexfoundry/south-sdk-go)](https://github.com/edgexfoundry/south-sdk-go/pulls) [![GitHub Contributors](https://img.shields.io/github/contributors/edgexfoundry/south-sdk-go)](https://github.com/edgexfoundry/south-sdk-go/contributors) [![GitHub Committers](https://img.shields.io/badge/team-committers-green)](https://github.com/orgs/edgexfoundry/teams/south-sdk-go-committers/members) [![GitHub Commit Activity](https://img.shields.io/github/commit-activity/m/edgexfoundry/south-sdk-go)](https://github.com/edgexfoundry/south-sdk-go/commits)


## Overview

This repository is a set of Go packages that can be used to build Go-based [node services](https://docs.edgexfoundry.org/2.1/microservices/node/Ch-NodeServices/) for use within the EdgeX framework.

## Usage

Developers can make their own node service by implementing the [`ProtocolDriver`](https://github.com/edgexfoundry/south-sdk-go/blob/main/pkg/interfaces/protocoldriver.go) interface for their desired IoT protocol, and the `main` function to start the Node Service. To implement the `main` function, the [`startup`](https://github.com/edgexfoundry/south-sdk-go/tree/main/pkg/startup) package can be optionally leveraged, or developers can write customized bootstrap code by themselves.

Please see the provided [simple node service](https://github.com/edgexfoundry/south-sdk-go/tree/main/example) as an example, included in this repository.

## Command Line Options

The following command line options are available

```text
  -cd=<path>
  --configDir=<path>
        Specify an alternate configuration directory.
  -p=<profile>
  --profile=<profile>
        Specify a profile other than default.
  -cf=<file>
  --configFile=<file>
        Indicates name of the local configuration file.
  -i=<instace>
  --instance=<instance>
        Provides a service name suffix which allows unique instance to be created.
        If the option is provided, service name will be replaced with "<name>_<instance>"
  -o
  --overwrite
        Overwrite configuration in the Registry with local values.
  -op 
  --overwriteProfiles
        Overwrite core-metadata with the versions of the associated node profile files.  
        *** Use with caution *** Use will clobber existing profiles in core-metadata, problematic 
        if those profiles were edited by hand intentionally or used by other services.
  -od
  --overwriteNodes
        Overwrite core-metadata with the versions of the associated node files.  
        *** Use with caution *** Use will clobber existing nodes in core-metadata, problematic 
        if those nodes were edited by hand intentionally.
  -r
  --registry
        Indicates the service should use the registry.
  -cp
  --configProvider
        Indicates to use Configuration Provider service at specified URL.
        URL Format: {type}.{protocol}://{host}:{port} ex: consul.http://localhost:8500
```

## Float value encoding

In EdgeX v1, float values had two kinds of encoding, [Base64](#base64), and [scientific notation (`eNotation`)](#scientific-notation-e-notation).

After v2, EdgeX only uses [scientific notation (`eNotation`)](#scientific-notation-e-notation) to present float values.

## Community

- Discussion: [https://github.com/orgs/edgexfoundry/discussions](https://github.com/orgs/edgexfoundry/discussions)
- Mailing lists: [https://lists.edgexfoundry.org/mailman/listinfo](https://lists.edgexfoundry.org/mailman/listinfo)

## License

[Apache-2.0](LICENSE)

## Versioning

Please refer to the EdgeX Foundry [versioning policy](https://wiki.edgexfoundry.org/pages/viewpage.action?pageId=21823969) for information on how EdgeX services are released and how EdgeX services are compatible with one another.  Specifically, node services (and the associated SDK), application services (and the associated app functions SDK), and client tools (like the EdgeX CLI and UI) can have independent minor releases, but these services must be compatible with the latest major release of EdgeX.

## Long Term Support

Please refer to the EdgeX Foundry [LTS policy](https://wiki.edgexfoundry.org/pages/viewpage.action?pageId=69173332) for information on support of EdgeX releases. The EdgeX community does not offer support on any non-LTS release outside of the latest release.
