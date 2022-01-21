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

from ops.charm import CharmBase
from ops.framework import StoredState
from ops.main import main
from ops.model import ActiveStatus

logger = logging.getLogger(__name__)


class K8PiclibCharm(CharmBase):
    """Charm the service."""

    _stored = StoredState()

    def __init__(self, *args):
        super().__init__(*args)
        self.framework.observe(self.on.piclib_server_pebble_ready, self._on_pebble_ready)
        self._stored.set_default(things=[])

    def _restart_piclib(self, container, libpath):
        binpath = self.model.resources.fetch("pics-binary")
        with open(binpath) as f:
            self.model.push('/pics', f, permissions=0o777)

        # Define an initial Pebble layer configuration
        pebble_layer = {
            "summary": "piclib layer",
            "description": "pebble config layer for piclib",
            "services": {
                "piclib": {
                    "override": "replace",
                    "summary": "piclib",
                    "command": "/pics -lib={} serve -all -addr={}".format(libpath, '0.0.0.0:7777'),
                    "startup": "enabled",
                }
            },
        }

        # Add initial Pebble config layer using the Pebble API
        container.add_layer("piclib", pebble_layer, combine=True)

        # Autostart any services that were defined with startup: enabled
        container.stop('piclib')
        container.autostart()

        # Learn more about statuses in the SDK docs:
        # https://juju.is/docs/sdk/constructs#heading--statuses
        self.unit.status = ActiveStatus()

    def _install_piclib(self, libpath):
        logger.debug('installing initial picture library')
        piclib_tar = self.model.resources.fetch("initial-piclib")
        os.makedirs(libpath, exist_ok=True)
        dstpath=os.path.join(libpath, 'piclib.tar')
        with open(piclib_tar) as f:
            self.model.push(dstpath, f, make_dirs=True)
        self.model.exec(['tar', '-xf', 'piclib.tar'], working_dir=libpath)

    def _on_pebble_ready(self, event):
        """Define and start a workload using the Pebble API.

        TEMPLATE-TODO: change this example to suit your needs.
        You'll need to specify the right entrypoint and environment
        configuration for your specific workload. Tip: you can see the
        standard entrypoint of an existing container using docker inspect

        Learn more about Pebble layers at https://github.com/canonical/pebble
        """
        # Get a reference the container attribute on the PebbleReadyEvent
        container = event.workload
        libpath = '/piclib'
        self._install_piclib(libpath)
        self._restart_piclib(container, libpath)


if __name__ == "__main__":
    main(K8PiclibCharm)
