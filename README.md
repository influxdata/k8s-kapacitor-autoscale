# Kapacitor + Kubernetes Autoscaling

This repository provides a simple example of how you can use [Kapacitor](https://www.influxdata.com/time-series-platform/kapacitor/) to autoscale resources in [Kubernetes](http://kubernetes.io/).
If you are already familiar with Kapacitor and Kubernetes you may just want to jump over to the [K8sAutoscale](https://docs.influxdata.com/kapacitor/v1.1/nodes/k8s_autoscale_node/) docs.

## Getting setup

The following tutorial walks you through autoscaling a simple application using [minikube](https://github.com/kubernetes/minikube).
The examples will also work if you have a complete k8s cluster already running, but we show the minikube commands to make this tutorial complete.

### Installing minikube

If you do not already have a running k8s cluster or minikube environment head over to their [installation guide](https://github.com/kubernetes/minikube#installation).

### Start minikube

Once you have minikube installed go ahead and start minikube:

    $ minikube start

### Get the repo

Download this repo and use it as a working directory:

    $ git clone https://github.com/influxdata/k8s-kapacitor-autoscale.git
    $ cd k8s-kapacitor-autoscale

## The Example Application

Our example application can be found in the `app` directory of this repo.
The app itself is extremely simple.
It does only two things:

1. On an HTTP GET request to its listening port it will return the current total number of requests the app has served.
2. Once per second it will send a point to Kapacitor using the [line protocol](https://docs.influxdata.com/influxdb/v1.1/write_protocols/line_protocol_tutorial/) containing the number of requests the app has served, tagged by host and k8s replicaset.

### Start the Application

This repo provides a basic k8s ReplicaSet definition for the app.
From the repository root run.

    $ kubectl create -f replicasets/app.yaml

Next expose the application as a service:

    $ kubectl create -f services/app.yaml


### Test the Application

Using minikube we can get an HTTP URL for our application:

    $ APP_URL=$(minikube service app --url)
    $ echo $APP_URL

Test that the app is working:

    $ curl $APP_URL/
    Current request count 1

## Start Kapacitor

Now that we have a working example app we can start up Kapacitor.
Similar to how we started the application we can create a ReplicaSet and Service for Kapacitor.
This time we do not need to build the image since the public `kapacitor` image will work just fine.

    $ kubectl create -f replicasets/kapacitor.yaml
    $ kubectl create -f services/kapacitor.yaml

### Test that Kapacitor is working

Again using minikube we can get a URL for connecting to Kapacitor

>NOTE: This time we export the URL so that the kapacitor client can also know how to connect.

    $ export KAPACITOR_URL=$(minikube service kapacitor --url)
    $ echo $KAPACITOR_URL

At this point you either need to have the `kapacitor` client installed locally or docker to run the client.
The client can be downloader from [here](https://www.influxdata.com/downloads/#kapacitor).

If you do not want to use the client locally then start a docker container.

    $ docker run -it --rm -v $(pwd):/k8s-kapacitor-autoscale:ro -e KAPACITOR_URL="$KAPACITOR_URL" kapacitor:1.1.0-rc2 bash

Once inside the container change directory to the repository:

    $ cd /k8s-kapacitor-autoscale


Now whether you are inside on container or on your local box the commands should be the same.

First check that we can talk to Kapacitor:

    $ kapacitor stats general

You should see output like the following:

    ClusterID:                    7b4d5ca3-8074-403f-99b9-e1743c3dbbff
    ServerID:                     94d0f5ea-5a57-4573-a279-e69a81fc5b5c
    Host:                         kapacitor-3uuir
    Tasks:                        0
    Enabled Tasks:                0
    Subscriptions:                0
    Version:                      1.1.0


### Using Kapacitor to autoscale our application

Kapacitor uses `tasks` to do work, the next steps involve defining a new task that will autoscale our app and enabling that task.
A task is defined via a [TICKscript](https://docs.influxdata.com/kapacitor/v1.1/tick/), this repository has the TICKscript we need, `autoscale.tick`.

Define and enable the autoscale task in Kapacitor:

    $ kapacitor define autoscale -tick autoscale.tick -type stream -dbrp autoscale.autogen
    $ kapacitor enable autoscale

To make sure the task is running correctly use the `kapacitor show` command:

    $ kapacitor show autoscale

There will be lots of output about the content and status of the task but the second to last line should look something like this:

    k8s_autoscale6 [avg_exec_time_ns="0s" cooldown_drops="0" decrease_events="0" errors="0" increase_events="0" ];

Since the task has just started the k8s_autoscale6 node has not processed any points yet but it will after a minute.

At this point take a minute to read the task and get a feel for what it is doing.
The high level steps are:

* Select the `requests` data that each application host is sending.
* Compute the requests per second for host
* For each replicaset, (in our case it's just the one `app` replicaset) compute the total request per second across all hosts.
* Compute a moving average of the total request per second over the last 60 points or 1m.
* Compute the desired number of hosts for the replicaset based on the target value.
    At this step Kapacitor will call out to the Kubernetes API and change the desired replicas to match the computed result.

There are some more details about cooldowns and other things, feel free to ignore those for now.

## Generate some load and watch the application autoscale

At this point our k8s cluster should have only two pods running, the one app pod and the one kapacitor pod.
Check this by list the pods:

    $ kubectl get pods

Once the request count increases on the app pod Kapacitor will instruct k8s to create more pods for that replicaset.
At that point you should see multiple app pods while still only seeing one Kapacitor pod.

There are several ways to generate HTTP requests, use a tool you are comfortable with.
If you do not already have a favorite HTTP load generation tool may we recommend [hey](https://github.com/rakyll/hey).
We also provide a simple script `ramp.sh` that uses `hey` to slowly ramp traffic up and then back down.

Install `hey` before running `ramp.sh`:

    $ go get -u github.com/rakyll/hey
    $ ./ramp.sh $APP_URL

### Watch autoscaling in progress

While the traffic is ramping up watch the current list of pods to see that more pods are added as traffic increases.
The default target is 100 requests per second per host.
The `ramp.sh` script will print out the current QPS it is generating, divide that number by 100 and round up.
That should be the number of app pods running.

    $ kubectl get pods -w

### Cooldown

You may have noticed that new nodes are not immediately added once a new threshold is crossed.
This is because we have instructed Kapacitor to only increase the number of replicas at most once per minute.
This is so that we give the new nodes that have been added a chance to warm up and for the cluster as a whole to stabilize.
Typically you would set this value to however long is takes a new pods to get up and running.
In our simple example the app can start up much faster because it does so little.
Feel free to play with the cooldown settings to see how it reacts.

## What next?

At this point you should be familiar with the basics of autoscaling a simple app using Kapacitor.
For more information and details on this process have a look at the [docs](https://docs.influxdata.com/kapacitor/v1.1/nodes/k8s_autoscale_node/)

## Why not just use an HPA?

Kubernetes already comes with a [horizontal pod autoscaler](http://kubernetes.io/docs/user-guide/horizontal-pod-autoscaling/) (HPA), why use Kapacitor?
First, off the HPA was the basis for how the autoscaler was implemented in Kapacitor.
Currently the HPA can only scale based off memory/cpu usage metrics or using custom metrics which must be defined via cAdvisor specific definitions.

By using Kapacitor you have access to a much richer set of data.
You can scale based off a combination of different metrics, aggregations of over multiple metrics, use historical traffic data or anything else you can dream up using TICKscripts.
This allows you to clearly define exactly what formula is being used for autoscaling, with visibility into each step of that processes.

