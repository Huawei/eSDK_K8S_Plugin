
# Huawei CSI Driver for Kubernetes
The Huawei CSI driver for Kubernetes or OpenShift.

## Compatibility
| Kubernetes Version |  |
|--|--|
| 1.13 | Yes |
| 1.14 | Yes |
| 1.15 | Yes |

## Compiling the driver
### Compiling requirements
| System | Go Version |
|--|--|
|Linux|>=1.10|

### How to compile
Create a **src** directory at any parent directory.
Put source code in this **src** directory. For instance the path tree looks like:

    .../src
			/csi
		    /dev
		    /proto
		    /storage
		    /tools
		    /utils
		    /vendor
		    /Makefile-CSI
**cd** to **src** directory, and run the following command:

    GOPATH=<absolute parent path of src> make -f Makefile-CSI PLATFORM=[X86|ARM]
| Argument | Description |
|--|--|
|GOPATH|The absolute parent path of src directory(Start from /)|
|PLATFORM|The target platform to compile for|

After the compilation is finished, a **bin** diretory will be created under **src**, where the executable programs will be placed in.
