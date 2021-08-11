# eSDK support CSI Topology-Aware Volume Provisioning with Kubernetes

**Author(s)**: [Amit Roushan](https://github.com/AmitRoushan)

## Version Updates
Date  | Version | Description | Author
---|---|---|---
Aug 5th 2021 | 0.1.0 | Initial design draft for storage topology support for eSDK kubernetes plugin| Amit Roushan 

## Terminology

 Term | Definition 
------|------
CSI | A specification attempting to establish an industry standard interface that Container Orchestration Systems (COs) can use to expose arbitrary storage systems to their containerized workloads.
PV | A PersistentVolume (PV) is a piece of storage in the cluster that has been provisioned by an administrator or dynamically provisioned using Storage Classes
PVC | A PersistentVolumeClaim (PVC) is a request for storage by a user. It is similar to a Pod. Pods consume node resources and PVCs consume PV resources.

## Motivation and background
Some storage systems expose volumes that are not equally accessible by all nodes in a
Kubernetes cluster. Instead volumes may be constrained to some subset of node(s) in the cluster.
The cluster may be segmented into, for example, “racks” or “regions” and “zones” or some other
grouping, and a given volume may be accessible only from one of those groups.

To enable orchestration systems, like Kubernetes, to work well with storage systems which expose
volumes that are not equally accessible by all nodes, the CSI spec enables:

- Ability for a CSI Driver to opaquely specify where a particular node exists with respect to
  storage system  (e.g. "node A" is in "zone 1").
- Ability for Kubernetes (users or components) to influence where a volume is provisioned
  (e.g. provision new volume in either "zone 1" or "zone 2").
- Ability for a CSI Driver to opaquely specify where a particular volume exists
  (e.g. "volume X" is accessible by all nodes in "zone 1" and "zone 2").

Kubernetes support this CSI abilities to make intelligent scheduling and provisioning decisions.

Being a CSI plugin, eSDK strive to support topological scheduling and provisioning for end customer.

## Goals
The document present detailed design to make eSDK enable for topological volume scheduling 
and provisioning in kubernetes cluster.

The design should
- Enable operator/cluster admin to configure eSDK for topological distribution
- Enable end user to provision volumes based on configured topology
- Add recommendations for topological name and configuration strategy 

### Non-Goals
The document will not explicitly define, provide or explain:
- kubernetes [Volume Topology-aware Scheduling](https://github.com/jsafrane/community/blob/master/contributors/design-proposals/storage/volume-topology-scheduling.md)
- [CSI spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) for topology support

### Assumptions and Constraints
- The document has only considered kubernetes as orchestrator/provisioner.
- Volume provisioning/scheduling over Kubernetes nodes are part of [kubernetes Volume Topology-aware Scheduling](https://github.com/jsafrane/community/blob/master/contributors/design-proposals/storage/volume-topology-scheduling.md).

### Input Requirements
Support topology awareness for eSDK
  
### Feature Requirements
- Enable operator/cluster admin to configure eSDK for topological distribution
- Enable end user to provision volumes based on configured topology
- Add recommendations for topological name and configuration strategy

#### Requirement Analysis
Support topology aware volume provisioning on kubernetes for eSDK plugin:
- Should work for pre-provisioned persistent volume (PV)
  - Cluster admin can create a PV with NodeAffinity which mean PV can only be accessed from Nodes that 
    satisfy the NodeSelector
  - Kubernetes ensures:
    - Scheduler predicate: if a Pod references a PVC that is bound to a PV with NodeAffinity, the predicate will 
      evaluate the NodeSelector against the Node's labels to filter the nodes that the Pod can be schedule to.
      Kubelet: PV NodeAffinity is verified against the Node when mounting PVs.
- Should work for Dynamic provisioned persistent volume (PV)
  - Dynamic provisioning aware of pod scheduling decisions, delayed volume binding must also be enabled
  - Scheduler will pass its selected node to the dynamic provisioner, and the provisioner will create a 
    volume in the topology domain that the selected node is part of.
- Operator/Cluster admin should
  - Able to specify topological distribution of kubernetes nodes
  - Able to provision topology aware dynamic or pre-provisioned persistent volume (PV)
  - Able to configure eSDK for topology aware volume provisioning
- Application developer/deployer should
  - Able to configure topology aware volume for workload

##### Functional Requirements
- Should work for pre-provisioned persistent volume (PV)
- Should work for Dynamic provisioned persistent volume (PV)
- Able to specify topological distribution of kubernetes nodes
- Able to provision topology aware dynamic or pre-provisioned persistent volume (PV)
- Able to configure topology aware volume for workload

##### Non Functional Requirements
- Should support old version of volume provisioning

### Performance Requirements
- Volume provisioned without topology remains performant

### Security Requirements
NA
### Other Non Functional Requirements (Scalability, HA etc…)
NA

## Architecture Analysis

### System Architecture

![System Architecture](resources/eSDKTopologyAwareness.jpg)

Kubernetes supports CSI specification to enable storage provider to write their storage plugin for volume provisioning.
Normally Kubernetes nodes are equally accessible for volume provisioning hence PV controller in controller manager trigger 
volume provisioning as soon as PV/PVC are defined. Therefore, scheduler cannot take into account any of the pod’s other
scheduling constraints. This makes it possible for the PV controller to bind a PVC to a PV or provision a PV with 
constraints that can make a pod unschedulable. 
Detail design of [Volume Topology-aware Scheduling](https://github.com/jsafrane/community/blob/master/contributors/design-proposals/storage/volume-topology-scheduling.md) is in purview of kubernetes
Summary:
- Admin pre-provisions PVs and/or StorageClasses.
- User creates unbound PVC and there are no prebound PVs for it.
- PVC binding and provisioning is delayed until a pod is created that references it.
- User creates a pod that uses the PVC.
- Pod starts to get processed by the scheduler.
- Scheduler processes predicates. the predicate function, will process both bound and unbound PVCs of the Pod. It will 
  validate the VolumeNodeAffinity for bound PVCs. For unbound PVCs, it will try to find matching PVs for that node 
  based on the PV NodeAffinity. If there are no matching PVs, then it checks if dynamic provisioning is possible for 
  that node based on StorageClass AllowedTopologies.
- After evaluation, the scheduler will pick a node.
- Schedule triggers volume provisioning by annotating PV with the selected nodes
- PV controller get informed by the event and start provisioning by passing selected node topogical info to external provisioner
- External provisioner eventually calls ```CreateVolume``` gRPC request to eSDK controller plugin with topological data.
- CSI controller plugin consumes topology data and provision volume accordingly.

The document handles eSDK adaption towards topological volume provisioning.
eSDK is centralized and split component CSI plugin. The component involves:
- ```Controller plugin``` communicates indirectly with Kubernetes master components and storage backend to 
  implement CSI Controller service functionality
- ```Node plugin``` communicates indirectly with Kubernetes master components adn storage backend to implement 
  node service functionality


## Detailed Design

Enabling topology aware provisioning with eSDK has the following aspects:
- Enabling Feature Gates in kubernetes
  - Topology aware volume scheduling is controlled by the VolumeScheduling feature gate, 
    and must be configured in 
    - kube-scheduler
    - kube-controller-manager
    - all kubelets.

- Set ```VOLUME_ACCESSIBILITY_CONSTRAINTS``` flag in ```GetPluginCapabilities``` call for identity service

- Make Kubernetes aware about topology
  - Enable eSDK node plugin to publish Specifies where (regions, zones, racks, etc.) the node is accessible from
    - Cluster admin MUST add topology aware labels for each node.
    
      Ex: topology.kubernetes.io/region or topology.kubernetes.io/zone
      - eSDK node plugin fetches node labels and pass on the topological data to kubelet in ```NodeGetInfo``` gRPC call
        ```
          NodeGetInfoResponse{
            node_id = "{HostName: k8s-node-1}"
            accessible_topology = 
                  {"topology.kubernetes.io/region": "R1", 
                    "topology.kubernetes.io/zone": "Z2"}
          }
        ```
    - Kubelet creates ```CSINodeInfo``` with topological data.
    - Same ```CSINodeInfo``` object is used during Volume provisioning/scheduling
      
      
- Topology aware volume provisioning:
  - Controller plugin responsible to make intelligent volume provisioning decision based on topological data 
    provided in ```CreateVolume``` CSI gRPC call
  - To support topology aware provisioning, configuration of different backend are provided with supported 
    topological distributions during deployment
    ```json
      csi.json: {
        "backends": [
            {
                "storage": "***",
                "name": "storage1",
                "urls": ["https://*.*.*.*:28443"],
                "pools": ["***"],
                "parameters": {"protocol": "iscsi", "portals": ["*.*.*.*"]}
                "supportedTopologies":[
                   {"topology.kubernetes.io/region": "R1", 
                      "topology.kubernetes.io/zone": "Z1"},
                ]  
            },
            {
                "storage": "***",
                "name": "storage2",
                "urls": ["https://*.*.*.*:28443"],
                "pools": ["***"],
                "parameters": {"protocol": "iscsi", "portals": ["*.*.*.*"]}
                "supportedTopologies":[
                   {"topology.kubernetes.io/region": "R1", 
                      "topology.kubernetes.io/zone": "Z2"},
                ]  
            }
        ]
    }
    ```
  - Topology aware provisioning by controller plugin:
    - External provisioner initiate volume provisioning by ```CreateVolume``` CSI gRPC call to controller plugin
    - External provisioner pass on ```accessibility_requirements``` parameter in ```CreateVolumeRequest```.
    - #### Scenario : ```accessibility_requirements``` parameter supplied:
      - Controller need to consider topological constraints provided in ```accessibility_requirements```
      - Controller can get topology attributes in two variants in ```accessibility_requirements```:
        - ```requisite```:
          - If requisite is specified, the volume MUST be accessible from at least one of requisite topologies
        - ```preffered```
          - MUST attempt to make the provisioned volume accessible using the preferred topologies in order from first to last
          - if requisite is specified, all topologies in preferred list <UST also be present in the list of requisite topologies
        - Controller need a new ```primaryFilterFuncs```
          - The filter function filer out StoragePool based on ```requisite``` and ```preffered``` parameter 
            of ```accessibility_requirements```
    - #### Scenario : ```accessibility_requirements``` parameter empty:
      - The call is generated when Volume provisioning do not care about topology (old behavior)
      - New defined filter function prioritize ```StoragePool``` without ```supportedTopologies``` as volume provisioned 
        are accessible from all the Kubernetes nodes.
      - if ```StoragePool``` without ```supportedTopologies``` not available, provisioned volume in one of StoragePool
        but provide ```supportedTopologies``` as ```accessible_topology ``` in ```CreateVolumeResponse``` from selected 
        StoragePool
    
    - #### Scenario: ```CreateVolume``` request with ```accessibility_requirements``` and ```VolumeSource``` info (Volume clone with topology )
      - eSDK extract out backend info from ```volume_id```.
      - New topology filter function, validate filtered backend with ```accessibility_requirements```.
      - Return error response if backend's ```supportedTopologies``` is not justified by ```accessibility_requirements```

  - ```GetCapacity``` CSI controller functionality is not supported in eSDK currently.
    - Need to consider topology attributes in ```GetCapacityRequest ``` if supported in future

- Enable user/workload to configure and use topology aware volume provisioning
  - A new StorageClass field BindingMode introduced to control the volume binding behavior
      ```text
          type StorageClass struct {
    
             BindingMode *BindingMode
          }

         type BindingMode string

         const (
            BindingImmediate BindingMode = "Immediate"
            BindingWaitForFirstConsumer BindingMode = "WaitForFirstConsumer"
        )
      ```
    - StorageClass introduces new binding behavior. The volume binging get delayed until a Pod is being scheduled 
    - ```BindingWaitForFirstConsumer``` binding mode should be used for topology aware volume provisioning
    
### Use case View
- A pod may need multiple PVCs. As an example, one PVC can point to a local SSD for fast data access, 
  and another PVC can point to a local HDD for logging. Since PVC binding happens without considering if multiple 
  PVCs are related, it is very likely for the two PVCs to be bound to local disks on different nodes, making the pod unschedulable.
- For multi backend clusters and deployments requesting multiple dynamically provisioned across zone, each PVC is provisioned 
  independently, and is likely to provision each PV in different zones, making the pod unschedulable
- Local storage PVC binding does not have any node spreading logic. So local PV binding will very likely conflict with 
  any pod anti-affinity policies if there is more than one local PV on a node


### Development and Deployment Context

#### Code

//Provide inputs for code structure, language, any open source code can be resused, coding methods, development env etc

#### Debug Model

//how to debug the module, specific logging, debug options etc…

  
#### Build & Package

//How this module is built along with other modules etc…What is the package model

#### Deployment
- Controller plugin should be scheduled to run on nodes sp that it can access all Storage backend
//How to install and deploy the module in the system, hardware resource requirements etc. Any other network or such requirements..like client or http server needed etc…

  
### Execution View

//During the run time, any specific aspects to be considered...like logging to be done for the module etc..It is not functional logs, it is specific to the module maintenance; OR Runtime replication or any such requirements to be considered during the design

  
## Sequence Diagrams

//Provide the key control and data flow sequence diagrams here

  
## Design Alternatives and other notes

//If you have any other ideas or alternate suggestions or notes which needs further analysis or later consideration, please add here
  

## Open Issues

NA

  
## Design Requirements / Tasks
- Make Kubernetes aware about topology
- Topology aware volume provisioning by controller plugin

## Scratchpad

//All raw inputs or discussion points or etc can be added here
