# Cosmos Operator Helm Chart

This helm chart installs and updates the Cosmos Operator.

## Important
* The chart is generated my the project level Makefile `make helm`. Do not hand edit any files except this README and Chart.yaml.
* The chart intentionally does not use a dash `-` in the name because Go templates [have issue with dashes](https://github.com/helm/helm/issues/2192) if used as a subchart.
