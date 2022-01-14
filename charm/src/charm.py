#!/usr/bin/env python3
# Copyright 2022 Robert Carlsen
# See LICENSE file for licensing details.
#
# Learn more at: https://juju.is/docs/sdk

"""Charm the service.

Refer to the following post for a quick-start guide that will help you
develop a new k8s charm using the Operator Framework:

    https://discourse.charmhub.io/t/4208
"""

import sys
import os
import signal
import stat
import logging
import tarfile
import time

from ops.charm import CharmBase, InstallEvent, StartEvent, ConfigChangedEvent
from ops.framework import StoredState
from ops.main import main
from ops.model import ActiveStatus, BlockedStatus

logger = logging.getLogger(__name__)

import subprocess
from subprocess import CalledProcessError

class PiclibCharm(CharmBase):

    _stored = StoredState()

    def __init__(self, *args):
        super().__init__(*args)
        self.framework.observe(self.on.start, self._on_start)
        self.framework.observe(self.on.config_changed, self._on_config_changed)

        self._stored.set_default(things=[], pid=-1)

    def _restart_piclib(self, libpath='/tmp/piclib'):
        pid = self._stored.pid
        if pid > 0:
            logger.debug('killing pics server with pid {}'.format(pid))
            os.kill(pid, signal.SIGTERM)
            time.sleep(1)
            self._stored.pid = -1

        # add execute permission to pics binary
        binpath = self.model.resources.fetch("pics-binary")
        binpath = self.model.resources.fetch("pics-binary")
        curr_perms = stat.S_IMODE(os.lstat(binpath).st_mode)
        os.chmod(binpath, curr_perms | stat.S_IXGRP | stat.S_IXUSR | stat.S_IXOTH)

        try:
            p = subprocess.Popen([binpath, "-lib", libpath, "serve", "-all", "-addr=0.0.0.0:7777"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            self._stored.pid = p.pid
        except CalledProcessError as e:
            # If the command returns a non-zero return code, put the charm in blocked state
            logger.debug("Starting pics server failed with return code %d:", e.returncode)
            logger.debug("    ", str(e))
            self.unit.status = BlockedStatus("Failed to start/enable pics gallery service")
            return

        # Everything is awesome
        self.unit.status = ActiveStatus()

    def _install_piclib(self):
        piclib_tar = self.model.resources.fetch("initial-piclib")
        libpath = '/piclib'
        os.makedirs(libpath, exist_ok=True)
        with tarfile.open(piclib_tar) as tar:
            tar.extractall(path=libpath)

    def _on_config_changed(self, event: ConfigChangedEvent) -> None:
        self._install_piclib()
        self._restart_piclib(libpath='/piclib')

    def _on_start(self, event: StartEvent) -> None:
        self._restart_piclib()

if __name__ == "__main__":
    main(PiclibCharm)
