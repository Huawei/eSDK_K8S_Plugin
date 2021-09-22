
# Building multi-architecture Docker images

**Author** : [Basabee Bora](https://github.com/itsbasabee)

## Motivation and Background

There are often circumstances when the platform we are using to build docker images is different from the platforms we want to target for deployment. For instance, building an image on Windows and deploying it to Linux and macOS machines would result in the container getting crashed with error message *exec format error*. This error indicates that the images architecture is different than the target machine, and therefore the image cannot run.

The obvious solution is to build the images on the machines on which they run. The better solution is to build images that can run on **multiple architectures**.

To mitigate this issue, Docker has introduced the concept of **multi-architecture (multi-arch) images**. A multi-arch image is a type of container image that may combine variants for different architectures, and sometimes for different operating systems. When running an image with multi-architecture support, container clients will automatically select an image variant that matches your OS and architecture.

## Prerequisites:
+ We expects the user to be familiar with basic docker commands along with it's functionalities.
+  Installation of miniumum docker version of **19.03**. Docker gained buildx support with version 19.03, so at least this version needs to be installed.

    The docker version can be checked with `docker --version` command.

+ Enable the experimental mode: Try running the `docker buildx` command, in case it is showing error saying *docker: ‘buildx’ is not a docker command*, we need to enable the **experimental mode**. This can be done in two ways:

  + By setting an environment variable :

        $ export DOCKER_CLI_EXPERIMENTAL=enabled

  + By turning the feature on in the config file *$HOME/.docker/config.json* :

        $ {
             ..............
             "experimental" : "enabled"
          }

   Once the experimental features are turned on, the same can be verified by:

        $ docker version
         Client: Docker Engine - Community
          …
         Experimental:      true
          …

+ We need a minimum  kernel version of **4.8** that supports the **binfmt_misc** feature. The binfmt_misc feature which is needed to use **QEMU** transparently inside containers is the **fix-binary (F) flag**.

   The kernel version can be checked with:

      $ uname -r

## Docker Images

Each docker image is represented by a **manifest**. A **manifest file** is a JSON file that instructs the docker daemon how to assemble the image for each architecture.

When building the multi-arch image, manifest file contains a list of manifests, so that the docker engine could pick the one that it matches at runtime.  This type of manifest is called a **manifest list**.

## Methods

There are two ways to use docker to build a multi-arch image:

+ using `docker manifest`, and
+ using `docker buildx`

## Doing it *docker manifest* way

To begin our journey, we’ll first need to build each architecture-specific image using `docker build` command and push to the Docker Hub.

Now that we have built our images and pushed them, we will be able to reference them all in a manifest list using the `docker manifest` command referencing the image with a tag.

    $ docker manifest create \
    your-username/multiarch-example:manifest-latest \
    --amend your-username/multiarch-example:manifest-amd64 \
    --amend your-username/multiarch-example:manifest-arm64v8

### Inspecting the result

The `docker manifest inspect` command shows the image manifest details for the multi-arch image showing the list of images it references and their respective platforms.

## What is *docker buildx* and why it's preferred?

The `docker buildx` is a docker feature that allows us :

 + Building an image for the native architecture, 
 + Supports emulation. 
 
 **Emulation** implies that from a specific machine we can build an image targeted for a different architecture-supported machines.

The preference of `docker buildx` over other approaches is backed by the following features 
:

+ No dedicated machines for building.
+ Configure and build the application on only one machine.
+ No separate Dockerfile per architecture.
+ No specific image name or image tag.

## How Docker Buildx Compiles for Non-Native Architectures?

The `docker buildx` build and compile images for different architectures using the **QEMU** processor emulator. 

QEMU executes all instructions of a foreign CPU on our host processor. E.g. it can simulate *ARM* instructions on an *x86* host machine. 

## Software Requirements for Buildx Non-Native Architecture Support

+ **Mounting of binfmt_misc file system**: The binfmt_misc kernel features are controlled via files in **/proc/sys/fs/binfmt_misc/**. This file system needs to be mounted such that userspace tools can control this kernel feature, i.e. **Register** and **enable handlers**.

  We can check if the file system is mounted with:

      $ ls /proc/sys/fs/binfmt_misc/
      register status

+ **Installation of QEMU and binfmt_misc support tools** :

    + **Installation of QEMU**: To execute foreign CPU instructions on the host, QEMU simulators need to be installed.

      An easy way to install QEMU binaries is to use a pre-built package for the host Linux distribution. For Debian or Ubuntu, it can be installed with:

          $ sudo apt-get install -y qemu-user-static
      
      This will install QEMU for a number of foreign architectures, the verification of which can be made by checking:

          $ ls -l /usr/bin/qemu-aarch64-static

          $ qemu-aarch64-static --version

      Other Linux distributions might use different package managers or package names for the QEMU package. Alternatively, we can install [QEMU from source](https://www.qemu.org/download/#source) and follow the build instructions.

    - **Installation of binfmt-support package**: We need to install a package of version **2.1.7 or newer** that contains an **update-binfmts** binary new enough to understand the fix-binary (F) flag and use it when registering QEMU simulators.

      When you perform the installation of qemu-user-static in the above step, it can be seen that it has automatically pulled in the recommended binfmt-support package. 
      
      If not installed, then manual installation of binfmt-support can be done with :


          $ sudo apt-get install -y binfmt-support
    
      Also, We need to make sure **update-binfmts** is installed with version **atleast 2.1.7**. The version can be checked with:

          $ update-binfmts --version

## Building Multi-Architecture Docker Images With Buildx

With all the software requirements on the host met, it’s time to explore how buildx is used to create multi-architecture docker images. The first step is setting up a buildx builder.

### Creating a Buildx Builder

Creation a new builder instance for buildx to use can be done with:

    $ docker buildx create --name <builder-name>
    $ docker buildx use <builder-name>

The status of newly created builder can be checked with:

    $ docker buildx ls

### Using Buildx

The `docker buildx build` subcommand has a number of flags which determine where the final image will be stored. By default, the resulting image will remain captive in **docker’s internal build cache** instead of local **docker images** list.

The **-- push** flag can be applied with `docker buildx build` command, which tells the docker to push the resulting image to a **docker registry**. The image tag has to contain the proper reference to the **registry** and **repository name**. This currently is the **best way** to store multi-architecture images.

After we successfully log in any docker registry, we can build and use the **--push** flag to push the image to Docker Hub. The architectures required to be supported can be specified with the **--platform** flag.

For e.g., 

     $ docker buildx build -t <name & tag in 'name:tag' format>
     --platform linux/amd64,linux/arm64 --push .

We can use the `imagetools` subcommand to inspect if required architecture versions are included in the image :

    $ docker buildx imagetools inspect <name & tag in 'name:tag' format>

The same can be verified on docker registray website logged in.

At the end, we can run the image to see final result:

    $ docker run --rm <name & tag in 'name:tag' format>

We can also inspect *local docker image* to verify if correct architecture version is being used or not:

    $ docker inspect --format “{{.Architecture}}” <name & tag in 'name:tag' format>

### Base Images and their supported architectures :

The supported architectures for base images can be checked by `docker buildx imagetools inspect <image-name>` command. This will display details of image in the registry.

For e.g.,

    $ docker buildx imagetools inspect busybox
    Name:      docker.io/library/busybox:latest
    MediaType: application/vnd.docker.distribution.manifest.list.v2+json
    Digest:    sha256:b37dd066f59a4961024cf4bed74cae5e68ac26b48807292bd12198afa3ecb778
           
    Manifests: 
    Name:      docker.io/library/busybox:latest@sha256:b862520da7361ea093806d292ce355188ae83f21e8e3b2a3ce4dbdba0a230f83
    MediaType: application/vnd.docker.distribution.manifest.v2+json
    Platform:  linux/amd64
             
    Name:      docker.io/library/busybox:latest@sha256:1128c48c4a1285628b24e32c50f70d8b03de0de9ffb27906746c3be842811894
    MediaType: application/vnd.docker.distribution.manifest.v2+json
    Platform:  linux/arm/v5

    ......................