#!/usr/bin/env python
import os
import sys
import yaml
import copy
import shutil
import socket
import argparse
import logging
import logging.config
from distutils.util import strtobool

import ansible_runner

sys.path.append('/opt/kubespray/contrib/inventory_builder/')
from inventory import KubesprayInventory

BASIC_LOGGING = {
    'version': 1,
    'formatters': {
        'default': {
            'format': '%(asctime)s %(levelname)s %(message)s',
        }
    },
    'handlers': {
        'default': {
            'level': 'DEBUG',
            'class': 'logging.StreamHandler',
            'formatter': 'default',
        }
    },
    'root': {
        'handlers': ['default'],
        'level': 'DEBUG',
    },
}

logging.config.dictConfig(BASIC_LOGGING)
LOG = logging.getLogger(__name__)


def parse_extra_vars(args_extra_vars_list):
    extra_vars = {}
    if not args_extra_vars_list:
        return extra_vars
    
    for args_extra_vars in args_extra_vars_list:
        for extra_var in args_extra_vars.split():
            k, v = extra_var.split('=')
            if v in ('True', 'true', 'false', 'False', 'yes', 'no'):
                v = bool(strtobool(v))
            extra_vars[k] = v
    return extra_vars


class JobChain(object):
    def __init__(self, config, skip_deployed_job):
        self.config = config
        self.skip_deployed_job = skip_deployed_job
        self.load()
    
    def load(self):
        with open(self.config) as f:
            self.jobs = yaml.load(f, Loader=yaml.FullLoader)
    
    def save(self):
        with open(self.config, 'w') as f:
            yaml.dump(self.jobs, f, indent=2)

    def run(self, inventory, extra_vars=None, limit=None):
        for job in self.jobs:
            if self.skip_deployed_job and job.get('rc', 255) == 0:
                LOG.info('{} has been deployed, skipped.'.format(job) )
                continue
            else:
                self.skip_deployed_job = False

            _j = copy.deepcopy(job)
            _ = _j.pop('rc', 255)
            _j['inventory'] = inventory
            if not _j.get('extravars'):
                _j['extravars'] = {}
            if isinstance(extra_vars, dict):
                _j['extravars'].update(extra_vars)
            if limit:
                _j['limit'] = limit

            LOG.info('Running job: {}'.format(_j))
            r = ansible_runner.run(**_j)
            job['rc'] = r.rc
            self.save()
            if job['rc'] != 0:
                sys.exit(r.rc)

def get_master_ip():
    s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    s.connect(("8.8.8.8", 80))
    master_ip = s.getsockname()[0]
    s.close()
    return master_ip

def build_inventory(args_host_vars, inventory_path):
    master_ip = get_master_ip()
    try:
        old_yaml_config = yaml.load(open(inventory_path, 'r'), Loader=yaml.FullLoader)
    except OSError:
        old_yaml_config = {}

    ki = KubesprayInventory(['master,{}'.format(master_ip)], inventory_path)
    ki.ensure_required_groups(['edgecore'])
    ki.write_config(ki.config_file)

    try:
        for ansible_hostname, host_vars in old_yaml_config['all']['hosts'].items():
            if ansible_hostname == 'master':
                continue
            ki.add_host_to_group('all', ansible_hostname, host_vars)
            ki.add_host_to_group('edgecore', ansible_hostname)
    except KeyError:
        pass

    if not args_host_vars:
        return
    
    for host_host_vars in args_host_vars:
        host_vars = {}
        for raw_host_vars in host_host_vars:
            k, v = raw_host_vars.split('=')
            host_vars[k] = v
        ansible_hostname = host_vars.pop('ansible_hostname')
        if ansible_hostname == 'master':
            ki.yaml_config['all']['hosts']['master'] = host_vars
        ki.add_host_to_group('all', ansible_hostname, host_vars)
        ki.add_host_to_group('edgecore', ansible_hostname)
    ki.write_config(ki.config_file)

def load_default_extra_vars(filename='default.yaml'):
    with open(filename) as f:
        return yaml.load(f, Loader=yaml.FullLoader)

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-e', '--extra-vars', type=str, nargs='*')
    parser.add_argument('--host-vars', type=str, nargs='*', action='append')
    parser.add_argument('-c', '--job-chain', type=str, choices=['deploy.yaml', 'edge-deploy.yaml'])
    parser.add_argument('--limit', type=str)
    parser.add_argument('-s', '--skip-deployed-job', default=True, type=lambda x: bool(strtobool(x)))
    parser.add_argument('-i', '--inventory', type=str, default='/etc/ansible/hosts')
    args = parser.parse_args()

    extra_vars = load_default_extra_vars()
    extra_vars.update(parse_extra_vars(args.extra_vars))
    build_inventory(args.host_vars, args.inventory)

    # https://serverfault.com/questions/630253/ansible-stuck-on-gathering-facts/865987
    if os.path.exists('/root/.ansible/cp'):
        shutil.rmtree('/root/.ansible/cp')

    jc = JobChain(args.job_chain, args.skip_deployed_job)
    jc.run(args.inventory, extra_vars, args.limit)
