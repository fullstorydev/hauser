# Deploying to Kubernetes

## Requirements
- kustomize
- kubectl

## Customize your deployment

Check a full dummy example [here](./example).

### Preparing your deployment files

1. Create a folder for your deployment files (e.g. `hauser`)
2. Add your hauser configuration file to the folder (e.g. `hauser/config.toml`)
3. Add a `.env` file to the folder containing your **secret** API token
    ```dotenv
    FULLSTORY_API_TOKEN="<my_secret_api_token>"
    ```
4. Create a `kustomization.yaml` file

    ```yaml
    namespace: fullstory

    resources:
    - github.com/fullstory/hauser/deploy/base/?ref=master
    
    images:
    - name: hauser
      newName: fullstory/hauser
      newTag: latest
    
    configMapGenerator:
    # configmap name needs to be exactly as bellow
    - name: fullstory-config
      files:
        - config.toml=config.toml
    
    secretGenerator:
    # secret name needs to be exactly as bellow
    - name: fullstory-api-token
      envs:
        - .env
    ```

The deployment can be further customizable with extra kubernetes resources you might want to add for a specific deployment environment (e.g. adding GCP/AWS credentials). Check out the [kustomize documentation](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/) to further extend the deployment.

### Deploying

```bash
kustomize build <your_deploy_folder> | kubectl apply -f -
```

or simply

```bash
kubectl apply -k <your_deploy_folder>
```
