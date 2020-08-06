#! /bin/bash

if [ -d /plan/ssh ]; then
  mkdir /root/.ssh
  cp /plan/ssh/* /root/.ssh
  chmod 700 /root/.ssh
  chmod 600 /root/.ssh/*
fi
