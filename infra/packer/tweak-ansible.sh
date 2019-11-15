#! /bin/bash

perl -pi -e 's/^#host_key_checking/host_key_checking/' /etc/ansible/ansible.cfg
perl -ni -e 'print $_;print "\ninterpreter_python = auto\n" if(/^\[defaults\]$/);' /etc/ansible/ansible.cfg
perl -nn -e 'print $_;print "enable_plugins = host_list, script, auto, yaml, ini, toml, aws_ec2\n" if(/^#enable_plugins = /);' /etc/ansible/ansible.cfg

