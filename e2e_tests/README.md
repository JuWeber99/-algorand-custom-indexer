# e2elive

An integration testing tool. It leverages an artifact generated by the go-algorand integration tests to ensure all features are exercised.

# Usage

A python virtual environment is highly encouraged.

## Setup environment with venv
```sh
cd e2e_tests

# create venv dir, I believe you could specify the version of python here
python3 -m venv venv_dir

# This command depends on your shell
source venv_dir/bin/activate.fish

# Install 'e2elive' and 'e2econduit'
python3 -m pip install .
```

## Running locally


### e2elive

This tests runs the `algorand-indexer` binary. Depending on what you're doing, you may want to use a different value for `--s3-source-net`. After the test finishes, a few simple queries are executed.

This command requires a postgres database, you can run one in docker:

```sh
docker run --rm -it --name some-postgres -p 5555:5432 -e POSTGRES_PASSWORD=pgpass -e POSTGRES_USER=pguser -e POSTGRES_DB=mydb postgres
```

Running e2elive

**Note**: you will need to restart this container between tests, otherwise the database will already be initialized. Being out of sync with the data directory may cause unexpected behavior and failing tests.
```sh
e2elive --connection-string "host=127.0.0.1 port=5555 user=pguser password=pgpass dbname=mydb" --s3-source-net "fafa8862/rel-nightly" --indexer-bin ../cmd/algorand-indexer/algorand-indexer --indexer-port 9890
```

### e2econduit

This runs a series of pipeline scenarios with conduit. Each module manages its own resources and may have dependencies. For example the PostgreSQL exporter starts a docker container to initialize a database server.
```sh
e2econduit --s3-source-net rel-nightly --conduit-bin ../cmd/conduit/conduit
```

## Run with docker

The e2elive script has a docker configuration available.

```sh
make e2e
```

You may need to set (and export) the `CI_E2E_FILENAME` environment variable.

```sh
export CI_E2E_FILENAME=rel-nightly
make e2e
```

Alternatively, modify `e2e_tests/docker/indexer/Dockerfile` by swapping out `$CI_E2E_FILENAME` with the test artifact you want to use. See `.circleci/config.yml` for the current values used by CI.

### Creating new Conduit e2e tests

## Creating a new plugin fixture
All plugins for e2e tests are organized in `e2e_tests/src/e2e_conduit/fixtures/` under the proper directory structure. For example, processor plugin fixtures are under `e2e_tests/src/e2e_conduit/fixtures/processors/`.

To create a new plugin, create a new class under the proper directory which subclasses `PluginFixture`.  
__Note that some fixtures have further subclasses, such as ImporterPlugin, which support additional required behavior for those plugin types.__

### Main Methods of PluginFixtures

* `name`  
Returns the name of the plugin--must be equivalent to the name which would be used in the conduit config.

* `setup(self, accumulated_config)`  
Setup is run before any of the config data is resolved. It accepts an `accumulated_config` which is a map containing all of the previously output config values.
If your plugin needs to be fed some data, such as the algod directory which was created by the importer, this data can be retrieved from the accumulated_config.

The setup method is responsible for any preparation your plugin needs to do before the pipeline starts.

* `resolve_config_input`  
This method sets values on `self.config_input`, a map which contains all of the Conduit config required to run your plugin. Any values set on this map will be serialized into
the data section of your plugin's config when running it in Conduit.

* `resolve_config_output`  
Here, similarly to `config_input`, we set values on the `config_output` map. This map is what will be passed between plugins during initialization via the `accumulated_config`. If your plugin is creating any resources or initializing any values which other plugins need to know about, they should be set in the `config_output`.


## Creating a new scenario using your plugin
A `Scenario` is an abstraction for a given instantiation of a Conduit pipeline. In order to run a coduit e2e test, create a scenario in `e2e_tests/src/e2e_conduit/scenarios` and ensure that it is run in `e2e_tests/src/e2e_conduit/e2econduit.py`.

For example,

```
Scenario(
        "app_filter_indexer_scenario",
        importer=importers.FollowerAlgodImporter(sourcenet),
        processors=[
            processors.FilterProcessor([
                {"any": [
                    {"tag": "txn.type",
                     "expression-type": "equal",
                     "expression": "appl"
                    }
                ]}
            ]),
        ],
        exporter=exporters.PostgresqlExporter(),
    )
```
