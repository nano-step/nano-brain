# Design: npm Scoped Package + CI Auto-Publish

## package.json Changes

```json
{
  "name": "@nano-step/nano-brain",
  "version": "2.0.0-beta.6",
  "publishConfig": {
    "access": "public"
  },
  "repository": {
    "type": "git",
    "url": "git+https://github.com/nano-step/nano-brain.git"
  },
  "license": "MIT"
}
```

## Release Workflow: Dual Publish

New `npm-publish` job in `release.yml`:

```yaml
npm-publish:
  needs: release
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: 20
        registry-url: https://registry.npmjs.org
    
    # Determine npm tag from git tag
    - name: Determine npm tag
      id: npm-tag
      run: |
        if [[ "${{ github.ref_name }}" == *"-beta"* ]]; then
          echo "tag=beta" >> $GITHUB_OUTPUT
        else
          echo "tag=latest" >> $GITHUB_OUTPUT
        fi
    
    # Set version from git tag (strip leading 'v')
    - name: Set version
      run: npm version --no-git-tag-version "${GITHUB_REF_NAME#v}"
    
    # Publish @nano-step/nano-brain
    - name: Publish @nano-step/nano-brain
      run: npm publish --tag ${{ steps.npm-tag.outputs.tag }} --access public
      env:
        NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
    
    # Publish nano-brain (unscoped alias)
    - name: Publish nano-brain
      run: |
        node -e "const p=require('./package.json'); p.name='nano-brain'; require('fs').writeFileSync('package.json',JSON.stringify(p,null,2))"
        npm publish --tag ${{ steps.npm-tag.outputs.tag }}
      env:
        NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

## Key Decisions

1. **Version from git tag**: `npm version --no-git-tag-version` sets version from tag, avoiding manual version bumps in package.json
2. **Dual publish order**: Scoped first (primary), then unscoped (alias)
3. **Unscoped alias**: Mutate `name` field in-place before second publish — simple, no extra files
4. **npm tag logic**: Any tag containing `-beta` → `--tag beta`, else → `--tag latest`
5. **No separate publish workflows**: Everything in `release.yml` — single source of truth
