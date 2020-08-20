# Secret Voting

This project was bootstrapped with [Create React App](https://github.com/facebook/create-react-app).

## Set up local servers....

To run this locally against a demo blockchain, you need to set up a few things:

CI server:

```sh
git clone https://github.com/CosmWasm/cosmwasm-js.git
cd cosmwasm-js
./scripts/wasmd/start.sh
```

A faucet to provide initial tokens ([see README](https://github.com/CosmWasm/cosmwasm-js/tree/master/packages/faucet)):

```sh
cd cosmwasm-js
cd packages/faucet
yarn dev-start
```

## Available Scripts

In the project directory, you can run:

### `node scripts/deploy_voting.js`
Uploads the contract code and instantiates 2 voting contracts

### `yarn start`

Runs the app in the development mode.<br />
Open [http://localhost:3000](http://localhost:3000) to view it in the browser.

The page will reload if you make edits.<br />
You will also see any lint errors in the console.

### `yarn test`

Launches the test runner in the interactive watch mode.<br />
See the section about [running tests](https://facebook.github.io/create-react-app/docs/running-tests) for more information.

### `yarn build`

Builds the app for production to the `build` folder.<br />
It correctly bundles React in production mode and optimizes the build for the best performance.

The build is minified and the filenames include the hashes.<br />
Your app is ready to be deployed!

See the section about [deployment](https://facebook.github.io/create-react-app/docs/deployment) for more information.

### `yarn eject`

**Note: this is a one-way operation. Once you `eject`, you can’t go back!**

If you aren’t satisfied with the build tool and configuration choices, you can `eject` at any time. This command will remove the single build dependency from your project.

Instead, it will copy all the configuration files and the transitive dependencies (webpack, Babel, ESLint, etc) right into your project so you have full control over them. All of the commands except `eject` will still work, but they will point to the copied scripts so you can tweak them. At this point you’re on your own.

You don’t have to ever use `eject`. The curated feature set is suitable for small and middle deployments, and you shouldn’t feel obligated to use this feature. However we understand that this tool wouldn’t be useful if you couldn’t customize it when you are ready for it.

## Learn More

You can learn more in the [Create React App documentation](https://facebook.github.io/create-react-app/docs/getting-started).

Also, here is the [React documentation](https://reactjs.org).