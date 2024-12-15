# WordPress Development CLI

This is a cli tool for developing WordPress themes and plugins.

**Features:**

- CI/CD
- Unit Testing
- Automatic updates

**CLI:**

```
wpx login --user=jhon
```

```
wpx new plugin awesome-plugin
cd awesome-plugin
wpx run

```

```
cd awesome-plugin
git add .; git commit -m "Hotfix #24"
wpx deploy --minor
```