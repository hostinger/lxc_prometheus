#!/usr/bin/python

import subprocess
from flask import Flask
app = Flask(__name__)

def run(command):
    p = subprocess.Popen(command, stdout=subprocess.PIPE, shell=True)
    return p.communicate()[0]

def containers():
    containers = []
    for lxc in run("lxc-ls --active -1").split("\n"):
        if lxc:
            containers.append(int(lxc.strip()))
    return containers

def stats(container):
    stats = {}
    for lxc in run("lxc-info -S -H -n %d" % container).split("\n"):
        if lxc:
            key, value = lxc.split(":")
            value = value.strip()
            stats[key.strip()] = int(value) if value.isdigit() else value
    return stats

def ips(container):
    ips = []
    for lxc in run("lxc-info -i -n %d" % container).split("\n"):
        if lxc:
            _, value = lxc.split("IP:")
            ips.append(value.strip())
    return ips

def info():
    info = []
    metrics = {
        'cpu': 'CPU use',
        'memory': 'Memory use',
        'total_bytes': 'Total bytes',
        'rx_bytes': 'RX bytes',
        'tx_bytes': 'TX bytes',
        'io': 'BlkIO use'
    }
    for lxc in containers():
       for metric, value in metrics.iteritems():
           info.append(to_prometheus(metric, lxc, ips(lxc)[1], stats(lxc)[value]))
    return info

def to_prometheus(metric, id, ip, value):
    return 'node_lxc_%s{id="%s", ip="%s"} %s\n' % (metric, id, ip, value)

@app.route("/")
def root():
    return "<h1>LXC metrics</h1><br><a href=/metrics>Metrics</a>"

@app.route("/metrics")
def metrics():
    return "".join(info()), 200, {'Content-type': 'text/plain'}

if __name__ == "__main__":
    app.run(host='::', port=9119)
