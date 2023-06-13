# Marble BackOffice

Marble BackOffice is a frontend for the staff of Marble. It displays information about clients and proposes a web interface for administrative tasks.

## Dependencies

- [Marble Backend API](https://github.com/checkmarble/marble-backend)
- Firebase Auth

## Development

### Dependencies

Main dev dependencies:

- node
- yarn

Main runtime dependencies:

- React ~18
- Mui Core ~5
- React router ~6

code style: Prettier

### Run locally

```
# installation
yarn
# run dev server
yarn dev --port=3000
```

### Deployment

### #Manual deployement in staging

⚠️ In the root directory `marble-backend`:

```
(cd marble-backoffice && yarn build) && firebase deploy --only hosting:marble-backoffice-staging
```


### Coding guidelines

Clean code architecture with minimal dependencies.

### Scaffolding

The goal is to re-scaffold the project in the future with different tech choices, so the amount of modification made to the scaffolded files stays low and documented (See "Scaffolding and customisations" in this file).

### Client side Routing

React router handle client side routing.

### Design system

Mui Core is used as an implemtation of material design.

A backoffice is made up of a lot of simple controls and pages. The choice to use Mui Core is driven by the need to write simple html, not ease of customization.

### Authentication

The authentication is handled by firebase.

The only supported Identity Provider is Google.

The official tutorial has been followed step by step: [feedbackAuthenticate Using Google with JavaScript](https://firebase.google.com/docs/auth/web/google-signin)

`firebase emulators:start`

### Firebase auth: signInWithRedirect

The authentication is using `signInWithRedirect`. The domain https://marble-backoffice-staging.web.app/organizations is registered in console.firebase.com > Authentication > Settings > Authorized domains

### Scaffolding and customisations

This project has been scaffolded using the following command:

```
yarn create vite --template react-swc-ts
```

Customisations made to the default project:

- add alias @ for ./src
