# docker-collector: Docker statistics collector for Unix containers

## Requirements

- Docker
- Go >= 1.4.2 (for developers)
- [Godep](https://github.com/tools/godep) (for developers)

## How to use

### With ElasticSearch

Run your local ElasticSearch using docker

```bash
docker run \
    --name docker-collector-elasticsearch \
	-d -p 9200:9200 -p 9300:9300 \
	elasticsearch:1.7.1 \
	elasticsearch
```

and then you can run `docker-collector` with a container with the following command:

```
docker run \
        -d \
        --name docker-collector \
        --privileged \
        -h "$(hostname)" \
        --pid host \
        -e ELASTIC_IP=ELASTIC_SEARCH_IP \
        -v /var/run/docker.sock:/var/run/docker.sock \
        cilium/docker-collector \
        -f 'docker-collector*'
```

#### Docker arguments' explanation

* `-d` - Detached mode: run the container in the background and print the new container ID.
* `--privileged` - Privileged mode: we need this mode in order for docker-collector to have access to all network interfaces and statistics in `/sys/class/net/*/statistics/`
* `-h` - Container hostname: we will set this container hostname with the same of the local host.
* `--pid` - host: use the host's PID namespace inside the container. We need in order to access the other's containers statistics.
* `-e ELASTIC_IP=ELASTIC_SEARCH_IP` - Environment variable used to communicate with the ElasticSearch. Since we are exposing the port 9200 (port used to transmit data) `docker-collector` will communicate to the given IP address, usually a local one is enough. (Note: It can't be 127.0.0.1 but it can be docker's bridge IP 172.17.42.1)
* `-v /var/run/docker.sock:/var/run/docker.sock` - Used to find which containers are running in the local host.

#### docker-collector arguments' explanation

Usage of /docker-collector/docker-collector-Linux-x86_64:
* `-c string` - Directory path for kibana configuration and or templates. Configuration filename: 'configs.json', template filename: 'templates.json' (default "/docker-collector/configs")
        (You can use docker `-v` option such as `-v ./myconfigs-directory-path:/docker-collector/configs` to use your own configuration files)
* `-d string` - Set database driver to store statistics, valid options are (elasticsearch) (default "elasticsearch")
* `-f string` - Regex option to prevent docker-collector from reading on those containers that are matched by the given regex. Example: docker-collector -f docker-*
* `-i string` - Use a specific the prefix of the index name for elasticsearch. Suffix is -YYYY-MM-DD (default "docker-collector")
* `-l string` - Set log level, valid options are (debug|info|warning|error|fatal|panic) (default "info")
* `-t uint` - Set refresh time (in seconds) to retrieve statistics from containers (default 60)

### Kibana

Now that you're running ElasticSearch and `docker-collector`, you can see all of then inside Kibana.

```
docker run \
    --name docker-collector-kibana \
    -e ELASTICSEARCH_URL=http://ELASTIC_SEARCH_IP:9200 \
    -p 5601:5601 \
    -d kibana:4.1.1
```

Open you browser under [kibana dashboard](http://127.0.0.1:5601/#/dashboard/docker-collector-dashboard?_g=(refreshInterval:(display:'30%20seconds',pause:!f,section:1,value:30000),time:(from:now-15m,mode:quick,to:now))&_a=(filters:!(),query:(query_string:(analyze_wildcard:!t,query:'*')),title:docker-collector-dashboard)) and you can see something similar to the following image.

 ![DockercollectorUI](./docs/dockercollectorUI.png)

Now you can see all the other containers except the ones starting with `docker-collector` in their name (see `-f` option used in docker-collector).

If you don't see any container in the kibana's dashboard start a simple container such as:

```
docker run --rm -ti ubuntu ping www.google.com
```

and you'll be able to see it in around 2 minutes since the default time used to put data in ElasticSearch is around 60 seconds.

## F.A.Q.

### What OS do you support?

Although we have made a docker image the `docker-collector` binary inside that image is only Linux/x86_64 ready.

## License

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.  You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the License for the specific language governing permissions and limitations under the License.
