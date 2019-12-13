# `Plan:` Chewing strategies for Large DataSets

![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square)

IPFS supports an ever-growing set of ways in how a File or Files can be added to the Network (Regular IPFS Add, MFS Add, Sharding, Balanced DAG, Trickle DAG, File Store, URL Store). This test plan checks their performance

## What is being optimized/measured (min/max, reach)

- (Minimize) Memory used when performing each of the instructions
- (Minimize) Time spent time chewing the files/directories
- (Minimize) Time spent pinning/unpinning & garbage collection
- (Minimize) Time spent creating the Manifest files using URL and File Store
- (Minimize) Number of leftover and unreferenceable nodes from the MFS root when adding multiple files to MFS
- (Minimize) Waste when adding multiple files to an MFS tree (nodes that are no longer referenceable from the MFS tree due to graph updates)

## Plan Parameters

- **Builder Parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Runner Parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Network Parameters**: Found on the [manifest](../manifests/chew-datasets.toml)

## Tests

### `Test:` IPFS Add Defaults

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
  - **Act I**
    - Add the files generated

### `Test:` IPFS Add Trickle DAG

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
  - **Act I**
    - Add the files generated

### `Test:`  IPFS Add Dir Sharding

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created with _sharding experiment enabled_
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
  - **Act I**
    - Add the directories generated

### `Test:` IPFS MFS Write

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
  - **Act I**
    - Add each file and directory using the files.write function
    - Output to total size of the IPFS Repo and size of the MFS root tree
  - **Act II**
    - Run GC
    - Output to total size of the IPFS Repo and size of the MFS root tree

### `Test:` IPFS MFS Dir Sharding

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created with _sharding experiment enabled_
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
    - _Enable Sharding Experiment_
  - **Act I**
    - Add each file and directory using the files.write function
    - Output to total size of the IPFS Repo and size of the MFS root tree
  - **Act II**
    - Run GC
    - Output to total size of the IPFS Repo and size of the MFS root tree

### `Test:` IPFS Url Store


- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created _with url store experiment enabled_
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
    - Set up an HTTP Server that serves the files created
  - **Act I**
    - Run Url Store on the HTTP Endpoint
    - Verify that all files are listed on the Manifest
  - **Act II**
    - Generate 10 more random files.
    - Run FileStore again
    - Verify that all files are listed on the Manifes

### `Test:` IPFS File Store

- **Test parameters**: Found on the [manifest](../manifests/chew-datasets.toml)
- **Narrative**
  - **Warm up**
    - The IPFS node/daemon is created _with file store experiment enabled_
    - If Online, connect to the other nodes running
    - Generate the Random Data that follows what was specificied by the params `File Sizes` and `Directory Depth`
  - **Act I**
    - `ipfs add --nocopy` on the folder with Random Data (the `--nocopy` will use the FileStore)
    - Verify that all files are listed on the Manifest with `ipfs filestore ls`
  - **Act II**
    - Generate 10 more random files.
    - `ipfs add --nocopy` again
    - Verify that all files are listed on the Manifest with `ipfs filestore ls`
