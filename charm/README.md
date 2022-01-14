# charm

use

```
juju deploy ./charm_ubuntu-20.04-amd64.charm --resource pics-binary=../cmd/pics/pics --resource initial-piclib=initial-piclib.tar
```

The pics binary must be built/linked statically.  The initial piclib tar must
untar all the piclib files directly into the destination/cwd.


## Description

TODO: Describe your charm in a few paragraphs of Markdown

## Usage

TODO: Provide high-level usage, such as required config or relations


## Relations

TODO: Provide any relations which are provided or required by your charm

## OCI Images

TODO: Include a link to the default image your charm uses

## Contributing

Please see the [Juju SDK docs](https://juju.is/docs/sdk) for guidelines 
on enhancements to this charm following best practice guidelines, and
`CONTRIBUTING.md` for developer guidance.
