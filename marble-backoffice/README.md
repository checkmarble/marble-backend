# Marble BackOffice

Marble BackOffice is a frontend for the staff of Marble. It displays information about clients and proposes a web interface for administrative tasks.

## Dependencies

- [Marble Backend API](https://github.com/checkmarble/marble-backend)

## Development

### Coding guidelines

Clean code architecture with minimal dependencies.

The goal is to re-scaffold the project in the future with different tech choices, so the amount of modification made to the scaffolded files stays low and documented (See "Scaffolding and customisations" in this file).

Main dev dependencies:
- node
- yarn

Main runtime dependencies:
- React ~18

### Run locally

```
# installation
yarn
# run dev server
yarn dev --port=3000
```

## Scaffolding and customisations

This project has been scaffolded using the following command:
```
yarn create vite --template react-swc-ts
```

Customisations made to the default project:

- none
