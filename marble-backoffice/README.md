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

## Deployment

⚠️ All firebase commands must be run In the root directory `marble-backend`:

### Deployment in Staging:

```
(cd marble-backoffice && yarn build-staging) && firebase deploy --project staging --only hosting
```

### Deployment in Production:
```
(cd marble-backoffice && yarn build-production) && firebase deploy --project production --only hosting
```


### Two firebase projects

- [`firebase project staging`](https://console.firebase.google.com/project/tokyo-country-381508/overview)
- [`firebase project prod`](https://console.firebase.google.com/project/marble-prod-1/overview)

<img width="585" alt="image" src="https://github.com/checkmarble/marble-backend/assets/130078989/be75687a-8bf6-4f13-8150-e1f8afb866c4">


## A firebase app and a deployment per project
 
Each firebase project contain a firebase app named `backoffice` used for authentication.

Each firebase project also contain a "site" in firebase hosting:

- staging site: [`https://marble-backoffice-staging.web.app`](https://console.firebase.google.com/project/tokyo-country-381508/hosting/sites/marble-backoffice-staging)
- production site: [`https://marble-backoffice-production.web.app`](https://console.firebase.google.com/project/marble-prod-1/hosting/sites/marble-backoffice-production)


## One firebase.json

`firebase.json` declares how the website is hosted for staging and production using the alias `backoffice`

```
"hosting": {
    "target": "backoffice",
   (...)
```

The alias `backoffice` is declared in `.firebaserc`.

## Firebase commands must specify --project

example:`firebase --project production hosting:sites:list`


## Technical design

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

`firebase --project staging emulators:start`

### Firebase auth: signInWithRedirect

The authentication is using `signInWithRedirect`. The domains are registered in console.firebase.com > Authentication > Settings > Authorized domains.

### Scaffolding and customisations

This project has been scaffolded using the following command:

```
yarn create vite --template react-swc-ts
```

Customisations made to the default project:

- add alias @ for ./src
