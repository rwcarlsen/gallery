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

import os
import stat
import logging
import tarfile
import time
import subprocess
import yaml

from subprocess import CalledProcessError
from ops.charm import CharmBase, StartEvent, ConfigChangedEvent
from ops.framework import StoredState, Object
from ops.main import main
from ops.model import ActiveStatus, BlockedStatus

logger = logging.getLogger(__name__)

class Reverter(Object):
    def __init__(self, charm, stored_state):
        super().__init__(charm, 'reverter')
        self._charm = charm
        self._stored = stored_state
        self._stored.set_default(config_history=[])

        charm.framework.observe(charm.on.revert_action, self._revert_action)
        charm.framework.observe(charm.on.config_changed, self._on_config_changed)

    def _on_config_changed(self, event: ConfigChangedEvent) -> None:
        cfg = self._charm.model._backend.config_get()
        self._stored.config_history.append(cfg)

    def _revert_action(self, event):
        snapshot = event.params['snapshot'] - 1
        cfg = self._stored.config_history[snapshot]._under
        print(yaml.dump(cfg))

class PiclibCharm(CharmBase):

    _stored = StoredState()

    def __init__(self, *args):
        super().__init__(*args)
        self.framework.observe(self.on.start, self._on_start)
        self.framework.observe(self.on.config_changed, self._on_config_changed)

        self._stored.set_default(things=[])

        self._reverter = Reverter(self, self._stored)

    def _restart_piclib(self, libpath):
        os.system("killall pics")
        logger.debug('killing pics server ')
        time.sleep(2)

        # add execute permission to pics binary
        binpath = self.model.resources.fetch("pics-binary")
        curr_perms = stat.S_IMODE(os.lstat(binpath).st_mode)
        os.chmod(binpath, curr_perms | stat.S_IXGRP | stat.S_IXUSR | stat.S_IXOTH)

        try:
            subprocess.Popen(
                [binpath, "-lib", libpath, "serve", "-all", "-addr=0.0.0.0:7777"], close_fds=True)
            logger.debug('started pics server')
        except CalledProcessError as e:
            # If the command returns a non-zero return code, put the charm in blocked state
            logger.debug("Starting pics server failed with return code %d:", e.returncode)
            logger.debug("    ", str(e))
            self.unit.status = BlockedStatus("Failed to start/enable pics gallery service")
            return

        # Everything is awesome
        self.unit.status = ActiveStatus()

    def _install_piclib(self, libpath):
        logger.debug('installing initial picture library')
        piclib_tar = self.model.resources.fetch("initial-piclib")
        os.makedirs(libpath, exist_ok=True)
        with tarfile.open(piclib_tar) as tar:
            tar.extractall(path=libpath)

    def _on_config_changed(self, event: ConfigChangedEvent) -> None:
        # TODO: don't restart the piclib unless it was already running.
        libpath = self.model.config['libpath']
        self._restart_piclib(libpath)

    def _on_start(self, event: StartEvent) -> None:
        libpath = self.model.config['libpath']
        self._install_piclib(libpath)
        self._restart_piclib(libpath)


if __name__ == "__main__":
    main(PiclibCharm)
