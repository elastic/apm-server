## Developer documentation

### ~Requirements

- Checkout `elastic/package-registry`, `elastic/package-storage` and `elastic/beats`
- Have `elastic/package-spec` at hand

### Guide

#### Update / fix a package

1. Actual work
    - Make changes in `apmpackage/apm` and/or code as needed
    - Run `make build-package`

2. Run the registry and stack
    - Run `docker-compose up -d`: this will run package-registry with the locally-built package mounted, and will
      start Elasticsearch and Kibana pointed at the local registry.

4. Test
    - Go to the Fleet UI, install the integration and test what you need. You generally will want to have a look at the
   installed assets (ie. templates and pipelines), and the generated `apm` input in the policy.
    - If you need to change the package, you *must* remove the installed integration first. You can use the UI
    or the API, eg: `curl -X DELETE -k -u elastic:changeme https://localhost:5601/abc/api/fleet/epm/packages/apm-0.1.0 -H 'kbn-xsrf: xyz'`
    See [API docs](https://github.com/elastic/kibana/tree/master/x-pack/plugins/fleet/dev_docs/api) for details.
    You normally don't need to restart the registry (an exception to this is eg. if you change a `hbs` template file).

5. Upload to the snapshot registry
    - When everything works and `apmpackage/apm/` changes have been merged to `master`, copy the new package to
    `package-storage/packages/apm/<version>` in the `package-storage` repo, `snapshot` branch.
    Do *NOT* override any existing packages. Instead, bump the qualifier version (eg: `0.1.0-dev.1` to `0.1.0-dev.2`)
    both in the folder name and the content (`manifest.yml` and `default.json` pipelines)
    - You can `cd script && python copy_package.py` for this.

#### Run the Elastic Agent

If you do code changes or a whole new version, you need to run the Elastic Agent locally.
Most of the work here is done in `beats/x-pack/elastic-agent`

0. Optional: Update the spec

   The spec informs whether the Elastic Agent should or should not start apm-server based on the policy file,
   and what settings to pass via GRPC call.
    - Edit `spec/apm-server.yml`
    - `mage update`

1. Build / Package

    *Always*
    - `mage clean`

    *First time*
    - `DEV=true PLATFORMS=darwin mage package` (replace platform as needed)
    - Untar `build/distributions` contents

    *Every time after*
    - `DEV=true mage build`
    - Copy `build/elastic-agent` to `build/distributions/elastic-agent-<version>-<platform>/data/elastic-agent-<hash>/`

    *Snapshots*
    - If you need the Elastic Agent to grab the snapshot apm-server artifact, prepend `SNAPSHOT=true` to the `mage` command
    - Note: as of 14/12/20 `SNAPSHOT=true mage package` is broken for some of us, but `SNAPSHOT=true mage build` works fine

2. Optional: Override policy / apm-server
    - Use the right `elastic-agent.yml` policy

      It might be one you just generated with the UI, or one you have at hand with an apm input.
      Copy to `build/distributions/elastic-agent-<version>-<platform>/elastic-agent.yml`

    - Override apm-server in `install` and `downloads` folders. Approximately:
      ```
      # compile apm-server
      cd ~/<path>/apm-server
      make && make update

      # tar and compress
      tar cvf apm-server-<stack-version>-<platform>.tar apm-server LICENSE.txt NOTICE.txt README.md apm-server.yml
      gzip apm-server-<stack-version>-<platform>.tar
      sha512sum apm-server-<stack-version>-<platform>.tar.gz | tee apm-server-<stack-version>-<platform>.tar.gz.sha512

      # delete old stuff
      cd ~/<path>/beats/x-pack/elastic-agent/build/distributions/elastic-agent-<version>-<platform>/data/elastic-agent-<hash>/downloads
      rm apm*
      rm -rf ../install/apm*

      # copy new files
      mv <path>/apm-server-<stack-version>-<platform>.tar* .
      mkdir -p ../install/apm-server-<stack-version>-<platform>
      tar zxvf apm-server-<stack-version>-<platform> -C ../install/apm-server-<stack-version>-<platform>
      ```
3. Run the Elastic Agent
    - `./build/distributions/<blablabla>/elastic-agent -e`
    - Check apm-server logs at `build/distributions/<blablabla>/data/<blablabla>/logs/default`

      (The last default in the path comes from the namespace in the policy)

#### Promote a package

Generally it should be done between FF and release.
1. Remove the qualifier version from the package
2. Push to the corresponding production branch(es)
