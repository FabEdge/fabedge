#!/usr/bin/env python

from __future__ import absolute_import, division, print_function
import os
import sys
import argparse
from subprocess import Popen, PIPE
import logging
import logging.config

try:
    import configparser
except:
    import ConfigParser as configparser


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


def _bytes2str(string):
    return string.decode('utf-8') if isinstance(string, bytes) else string


def run_command_return_rc(command, **kwargs):
    command = ' '.join(command)
    process = Popen(command, stdout=PIPE, stderr=PIPE, shell=True, **kwargs)
    process.communicate()
    return process.returncode


def run_command(command, raise_exception=False, **kwargs):

    command = ' '.join(command)

    LOG.debug('Running command: {}'.format(command))
    process = Popen(command, shell=True, stdout=PIPE, stdin=PIPE, stderr=PIPE, **kwargs)
    out, err = process.communicate()

    if process.returncode == 0:
        return _bytes2str(out), _bytes2str(err), process.returncode

    err_msg = '"{}" failed with err: {}'.format(command, err)
    LOG.error(err_msg)
    if raise_exception:
        raise RunCommandError(err_msg)
    return _bytes2str(out), _bytes2str(err), process.returncode


class RunCommandError(Exception):
    pass


class InvalidSection(Exception):
    pass


class InvalidPolicy(Exception):
    pass


class InvalidTable(Exception):
    pass


TABLE_CHAINS = {
    'filter': ('INPUT', 'OUTPUT', 'FORWARD'),
    'nat': ('PREROUTING', 'POSTROUTING', 'OUTPUT'),
    'mangle': ('PREROUTING', 'OUTPUT', 'FORWARD', 'INPUT', 'POSTROUTING'),
    'raw': ('PREROUTING', 'OUTPUT')
}

POLICY = ('ACCEPT', 'DROP', 'QUEUE', 'RETURN')


class IPTablesPersistent(object):
    def __init__(self, config):
        self.tables = {}
        self.load_config(config)

    def validate_section(self, section):
        if section.count(":") != 1:
            raise(InvalidSection, section)

    def load_config(self, config):
        conf = configparser.ConfigParser(allow_no_value=True)
        conf.optionxform = str
        conf.read(config)

        for section in conf.sections():
            LOG.debug('Loading {}.'.format(section))
            try:
                self.validate_section(section)
            except InvalidSection as e:
                LOG.error(e)
                continue

            table_name, chain_name = section.split(':')
            table = self.tables.get(table_name)
            if not table:
                try:
                    self.tables[table_name] = Table(table_name)
                except InvalidTable as e:
                    LOG.error(e)
                    continue
                table = self.tables[table_name]

            chain = table.chains.get(chain_name)
            if not chain:
                chain = table.chains[chain_name] = Chain(chain_name, table)

            for key, value in conf.items(section):
                if key == 'policy':
                    if chain.name in TABLE_CHAINS[table.name]:
                        chain.policy = value
                    else:
                        LOG.error("Bad built-in chain name: {}".format(chain.name))
                    continue
                # https://stackoverflow.com/questions/21328509/config-parser-choosing-name-and-value-delimiter
                if value:
                    key = "{}:{}".format(key, value)
                chain.rules.append(key)

    def start(self):
        for table in self.tables.values():
            # user-defined
            for chain in table.chains.values():
                if chain.name not in TABLE_CHAINS[table.name]:
                    table.create_chain(chain.name)
                    for rule in chain.rules:
                        chain.append_rule(rule)
            # builtin
            for chain in table.chains.values():
                if chain.name in TABLE_CHAINS[table.name]:
                    for rule in chain.rules:
                        chain.append_rule(rule)
                    if chain.policy:
                        chain.set_policy(chain.policy)

    def stop(self):
        for table in self.tables.values():
            for chain in table.chains.values():
                if chain.policy:
                    chain.set_policy('ACCEPT')
                for rule in chain.rules:
                    chain.delete_rule(rule)


class Table(object):
    def __init__(self, name):
        self._name = None
        self.name = name
        self.chains = {}
    
    @property
    def name(self):
        return self._name

    @name.setter
    def name(self, value):
        if value not in TABLE_CHAINS.keys():
            raise InvalidTable('invalid table: {}'.format(value))
        self._name = value

    def create_chain(self, name):
        if run_command_return_rc(['iptables', '-t', 'filter', '-L', name]) != 0:
            run_command(['iptables', '-t', self.name, '-N', name])

    def get_chain(self, name):
        self.chains[name] = self.chains.get(name, Chain(name, self))
        return self.chains[name]

class Chain:

    def __init__(self, name, table):
        self.table = table
        self.name = name
        self.rules = []
        self._policy = None

    @property
    def policy(self):
        return self._policy

    @policy.setter
    def policy(self, value):
        if value not in POLICY:
            raise InvalidPolicy('invalid policy: {}, choice from: {}'.format(value, POLICY))
        self._policy = value

    def delete_rule(self, rule):
        if self.check_rule(rule):
            run_command(['iptables', '-t', self.table.name, '-D', self.name, rule])

    def append_rule(self, rule):
        if not self.check_rule(rule):
            run_command(['iptables', '-t', self.table.name, '-A', self.name, rule])

    def set_policy(self, policy):
        run_command(['iptables', '-t', self.table.name, '-P', self.name, policy])

    def check_rule(self, rule):
        command = ['iptables', '-t', self.table.name, '-C', self.name, rule]
        return True if run_command_return_rc(command) == 0 else False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--config', default='/etc/edge/iptables-persistent/iptables.ini')
    parser.add_argument('action', choices=['start', 'stop'])
    args = parser.parse_args()

    if not os.path.exists(args.config):
        LOG.error("cannot access '{}': No such file or directory".format(args.config))
        sys.exit(1)

    i = IPTablesPersistent(args.config)
    if args.action == 'start':
        i.start()
    if args.action == 'stop':
        i.stop()
main()
