#!/bin/bash
# Generate dataset.json from template by replacing __PROJECT__ with the real project name.
# Usage: ./generate-dataset.sh <project-name>
# Example: ./generate-dataset.sh tradeit
PROJ="${1:?Usage: $0 <project-name>}"
sed "s|__PROJECT__|${PROJ}|g" dataset.template.json > dataset.json
echo "Generated dataset.json with project prefix: $PROJ"
