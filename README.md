CoGS - Cooperative GPU sharing
==============================

CoGS is a client-server mechanism for managing GPU (CUDA) allocations on a multi GPU system that does not have some sort of fancy abstraction mechanism built-in. At the moment the goal is to serve our very specialized setup (each user has a Docker container), but can be made more general in the future.

The CoGS daemon monitors status of CUDA devices using SMI, it checks which processes are using these devices and identifies users (users are deterimend from labels of corresponding containers).

The client is used to access the usage information from within the containers, it can set different claims to individual devices. Claims can be enforced by killing processes that belong to other users.


Compiling
---------

In theory, all you have to do is run `make` and you will get binaries for server and client in a `bin` directory.

Usage
-----

TODO

