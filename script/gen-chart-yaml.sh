#!/bin/sh

set -eu

cat > helm/charts/cosmosoperator/Chart.yaml <<EOL
# This Chart.yaml is generated. Do not edit. See script/gen-chart-yaml.sh.
apiVersion: v2
name: cosmosoperator
description: A Helm chart for Kubernetes. Installs the Cosmos Operator.
type: application

# This is the chart version. This version number should be incremented each time you make changes
# to the chart and its templates, including the app version.
# Versions are expected to follow Semantic Versioning (https://semver.org/)
version: ${CHART_VERSION}

# This is the version number of the application being deployed. This version number should be
# incremented each time you make changes to the application. Versions are not expected to
# follow Semantic Versioning. They should reflect the version the application is using.
# It is recommended to use it with quotes.
appVersion: "${APP_VERSION}"
EOL