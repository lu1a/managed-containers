# managed-containers
Managed containers/db/objsto with a web UI. BYO hardware.

## Project structure

This Go app will sit outside of my multi-tenant kubernetes cluster "zones" (a cluster in a given city).
It will perform operations on one or more of the clusters depending on API requests from users.
It will enforce "claims" built on top of containers in quota-limited resource chunks.

A user will give a link to an container registry docker image, and this app handles everything from setting up a pod which runs that image, to hosting it on a random subdomain of one LCaaS website which a user can just proxy to from their own web domain. If they want more power in their running container, or more containers, it'll be deducted from their resource quota which is shared evenly between all users. Every user will pay a flat fee monthly, or maybe the users just donate hardware into the clusters and don't pay anything. The point of this is that the service won't be for profit - the users together get a share of the computing pie (maybe with some extra charge so that there can be new computers added on demand).

In the future there will be a multi-tenant object storage service as well, and this go app will likely handle the lifecycle of users within.

## TODO

- Fullstack work:
  - Harden all frontend form fields
- Backend work:
  - Containers: functionality for updating a running container
  - Containers: config for KNative, for serverless applications
  - Projects: Ability to create API tokens on project level
  - DB: GB limits on project db, with option to upgrade
  - Containers: create reverse proxy in front of zones which route from 80 to different services on different workers
  - Reconciler: intermittently check that the claims map to running services
  - Security: test images which try to break out of RunC and get host shell access
  - Object storage: initial backend boilerplate for seaweedfs
  - Object storage: make it work all in all
  - Containers: Different env vars for the instances in different zones
- Frontend work:
  - Make UI look good

## Links for me to re-read

[https://kubernetes.io/docs/concepts/security/multi-tenancy/](https://kubernetes.io/docs/concepts/security/multi-tenancy/)
(I'm thinking only _namespace isolation_ + _user network isolated into namespace_ + _resource quota per namespace? or per pod?_)
