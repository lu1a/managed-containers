## Postgres (DBaaS) setup

After creating a postgres instance, run these commands:
```
> \c template1;
> REVOKE ALL ON SCHEMA PUBLIC FROM PUBLIC;
> REVOKE CONNECT ON DATABASE postgres FROM PUBLIC;
```