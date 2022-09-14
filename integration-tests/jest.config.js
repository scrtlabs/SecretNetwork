/** @type {import('ts-jest/dist/types').InitialOptionsTsJest} */
module.exports = {
  preset: "ts-jest",
  testEnvironment: "node",
  testTimeout: 60_000,
  verbose: true,
  // modulePathIgnorePatterns: ["dist", "scripts"],
  // globalSetup: "<rootDir>/test/globalSetup.ts",
  // globalTeardown: "<rootDir>/test/globalTeardown.js",
  // setupFilesAfterEnv: ["<rootDir>/test/setup.ts"],
  //   globals: {
  //     TEST_ACCOUNTS: {},
  //   },
};
