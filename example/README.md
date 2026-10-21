# node-simple

The included node-simple example node service demonstrates basic usage of south-sdk-go.

## Protocol Driver

To make a functional Node Service, developers must implement the [ProtocolDriver](../pkg/interfaces/protocoldriver.go) interface. 
`ProtocolDriver` interface provides abstraction logic of how to interact with Node through specific protocol. See [simpledriver.go](driver/simpledriver.go) for example.

## Node Discovery

Some node protocols allow for nodes to be discovered automatically.
A Node Service may include a capability for discovering nodes and creating the corresponding Node objects within EdgeX.  

To enable node discovery, developers need to implement the `Discover` method which is used to trigger protocol-specific node discovery.
Any nodes found as a result of discovery being triggered are returned to the SDK via a go channel, passed to the implementation as a parameter during Initialization.
New discovery attempts may be started as soon as a slice of nodes is submitted, so in oreder to avoid the service being congested by concurrent discovery.
  
The SDK will then filter these nodes against pre-defined acceptance criteria (i.e. Provision Watchers), and add any nodes which match (excluding existing nodes).

A Provision Watcher contains the following fields:

`Identifiers`: A set of name-value pairs against which a new node's ProtocolProperties are matched  
`BlockingIdentifiers`: An additional set of name-value pairs that, if matched, will block the addition of a newly discovered node  
`ProfileName`: The name of a Profile which should be assigned to new nodes which meet the given criteria  
`ServiceName`: The name of a NodeService which the ProvisionWatcher should be applied on  
`AdminState`: The initial Administrative State for new nodes which meet the given criteria  
`AutoEvents`: A list of AutoEvent associated with the newly discovered node 
 
A candidate new node passes a ProvisionWatcher if all the Identifiers match, and none of the Blocking Identifiers match.
For nodes with multiple `Node.Protocol`, each `Node.Protocol` is considered separately. A match on any of the protocols results in the node being added.

Finally, A boolean configuration value `Node/Discovery/Enabled` defaults to false. If it is set true, and the DS implementation supports discovery, discovery is enabled.
Dynamic Node Discovery is triggered either by internal timer(see `Node/Discovery/Interval` in [configuration.yaml](cmd/node-simple/res/configuration.yaml)) or by a call to the node service's `/discovery` REST endpoint.

The following steps show how to trigger discovery on node-simple:
1. Set `Node/Discovery/Enabled` to true in [configuration file](cmd/node-simple/res/configuration.yaml)
2. Trigger discovery by sending POST request to DS endpoint: http://edgex-node-simple:59999/api/v3/discovery
3. `Simple-Node02` will be discovered and added to EdgeX, while `Simple-Node03` will be blocked by the [Provision Watcher `BlockingIdentifiers`](cmd/node-simple/res/provisionwatchers/Simple-Provision-Watcher.yml)

## Extended Protocol Driver
### ProfileScan
Some node protocols allow for nodes to discover profiles automatically.
A Node Service may include a capability for discovering node profiles and creating the associated Node Profile objects for nodes within EdgeX.

To enable profile scan, developers need to implement the [ProfileScan](../pkg/interfaces/protocolprofile.go) interface.
The `ExtendedProtocolDriver` interface defines a `ProfileScan` method which is used to trigger node-specific profile scan.
The node profile found as a result of profile scan is returned to the SDK, and the SDK will then create the node profile and update the node to use the profile.

The following steps show how to trigger profile scan on node-simple:
1. A pre-define node `ProfileScan-Simple-Node` is created without an associated node profile.
2. Trigger profile scan by sending POST request to DS endpoint: http://edgex-node-simple:59999/api/v3/profilescan with payload:
   ```json
   {
     "apiVersion": "v3",
     "nodeName": "ProfileScan-Simple-Node",
     "profileName": "ProfileScan-Test-Profile"
   }
   ```
3. Node Profile `ProfileScan-Test-Profile` will be added to EdgeX and `ProfileScan-Simple-Node` will be updated to use the node profile.

### StopNodeDiscovery and StopProfileScan
The `ExtendedProtocolDriver` interface defines a `StopNodeDiscovery` to stop the node discovery and `StopProfileScan` to stop the profile scanning.
