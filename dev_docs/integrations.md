# Integrations Testing
This documents covers the steps required to make changes to the APM package in Elastic Integrations and validate using a local Elastic Stack and a local Elastic Agent.

## Use Cases
1. Expose new APM-Server configuration to the APM Integration 

## Update Integrations
1. Follow the steps [here](https://github.com/elastic/integrations/blob/main/CONTRIBUTING.md) to fork and clone the integrations repo and make your changes 
2. Use [elastic-package](https://github.com/elastic/elastic-package) to spin up a local stack. See integrations [quick start](https://www.elastic.co/docs/extend/integrations/quick-start) guide for more details
- Run the below command. This will create a local stack (Kibana, Elasticsearch, Fleet Server, etc). This will also output a couple links to the local Kibana and Elasticsearch instances. It will also contain credentials to log in to the local Kibana.
    
    ```bash
    elastic-package stack up -d --version 9.0.0
    ```
   
3. Install the updated APM package 
- Update the `apm` integration as needed. Then run the below commands to install the updates on the Fleet Server.
    
    ```bash
    cd packages/apm
    elastic-packge build
    elastic-package install
    ```
   
4. Install the integration using the Kibana UI
- Go to the Kibana Host: https://127.0.0.1:5601.
- Then to Integrations > Installed integrations > Elastic APM.
- Select `Add Elastic APM` and configure the integration/policy.
- Select `Save and continue` and `Add Elastic Agent to your hosts`. Then select `Add agent`.
- Copy the provided commands to download and install the Elastic Agent.

## Install an Elastic Agent
1. Install the agent in a Docker container
- Note: You can install the agent on your local instead with the `--develop` option as described [here](https://github.com/elastic/elastic-agent?tab=readme-ov-file#development-installations).
- Start the container.

    ```bash 
    docker run --rm -it buildpack-deps:24.04 /bin/bash
    ```
- Next we need to add a dns bypass entry for `fleet-server` since the Elastic-Agent will use this hostname to connect to the Fleet Server in the local stack.
- Run the below command and copy the ipaddress.

    ```bash
    curl -vv host.docker.internal
    ```
    
- Then run the below command with the updated ipaddress.

    ```bash
    echo "<ipaddress> fleet-server" >> /etc/hosts
    ```
  
2. Run the below commands within the container
- This is the command provided in Kibana when you select `Add agent`. The example below has been modified for ARM and includes the required `--insecure` option.
    
    ```bash 
    curl -L -O https://artifacts.elastic.co/downloads/beats/elastic-agent/elastic-agent-9.0.1-linux-arm64.tar.gz 
    tar xzvf elastic-agent-9.0.1-linux-arm64.tar.gz
    cd elastic-agent-9.0.1-linux-arm64
    ./elastic-agent install --url=https://fleet-server:8220 --enrollment-token=enE0al81WUJLeTNtNjR0ZEVLNDI6Rm5ZY1pPcXl2Q3lzeWx5T0FmYVdiQQ== --install-servers --insecure
    ```
   
3. Run `elastic-agent status`. You should see a output like this:
    
    ```
    ┌─ fleet
    │  └─ status: (HEALTHY) Connected
    └─ elastic-agent
       └─ status: (HEALTHY) Running
    ```
    
- You can also double-check the agent is healthy in Kibana under Fleet > Agents.
4. The agent is now installed and running within the docker container. You can now send data to the APM server and perform any validations