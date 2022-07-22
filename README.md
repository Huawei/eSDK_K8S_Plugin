
# 广汽自动驾驶项目--华为CSI
## 说明

本分支代码仅供广汽自动驾驶项目使用，代码分支基于华为CSI V2.2.16，StorageClass中新增三个可配置参数：unixPermission、rootSquash、allSquash。

## huawei-csi二进制编译
### 编译环境
| System | Go Version |
|---|---|
|Linux|    >=1.17|

### 编译

1. 下载源代码，并进入到Makefile所在的目录下
2. 执行编译命令
    ```
    make -f Makefile RELEASE_VER=[2.3.RC4] VER=[2.2.16.1] PLATFORM=[X86|ARM]
    ```

3. 使用bin目录下的huawei-csi二进制制作镜像，详细操作请参考docs文档。
    ```
    - bin
       - huawei-csi
       - secretGenerate
       - secretUpdate
    ```

## 新增参数说明

在使用Pacific NAS存储时，支持在StorageClass的配置文件中配置如下三个参数：

- unixPermission：命名空间根目录UNIX权限
- rootSquash：root权限限制
- allSquash：权限限制

### 使用示例

```
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: mysc-per
provisioner: csi.huawei.com
parameters:
  volumeType: fs
  allocType: thin
  authClient: "*"
  unixPermission: "777"
  rootSquash: "1"
  allSquash: "1"
```

### 示例参数说明

以下三个参数仅支持配置为字符串格式

#### unixPermission

说明：命名空间根目录UNIX权限

参数配置格式：与Linux环境中权限配置格式一致，如"777"

不配置该参数则默认为755

#### rootSquash

说明：设置是否允许客户端的root权限。

可配置值："0"，"1"

"0"：root_squash：表示不允许客户端以root用户访问，客户端使用root用户访问时映射为匿名用户。
"1"：no_root_squash：表示允许客户端以root用户访问，保留root用户的权限。

不配置该参数则默认为"1" no_root_squash

#### allSquash

说明：设置是否保留共享目录的UID和GID。

可配置值："0"，"1"

"0"：all_squash：表示共享目录的UID和GID映射为匿名用户。
"1"：no_all_squash：表示保留共享目录的UID和GID。

不配置该参数则默认为"1" no_all_squash

