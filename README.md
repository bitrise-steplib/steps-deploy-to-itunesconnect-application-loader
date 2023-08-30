# Deploy to App Store Connect - Application Loader (formerly iTunes Connect)

[![Step changelog](https://shields.io/github/v/release/bitrise-steplib/steps-deploy-to-itunesconnect-application-loader?include_prereleases&label=changelog&color=blueviolet)](https://github.com/bitrise-steplib/steps-deploy-to-itunesconnect-application-loader/releases)

Uploads binaries (.ipa / .pkg files) to [App Store Connect](https://appstoreconnect.apple.com/).

<details>
<summary>Description</summary>

Upload your binaries to [App Store Connect](https://appstoreconnect.apple.com/) using Apple's Application Loader. You can upload iOS, macOS, or Apple TV apps with the Step. The Step does not upload metadata, screenshots, nor does it submit your app for review. For that, use the **Deploy to App Store Connect with Deliver** Step.

This Step, however, does NOT build your binary: to create an IPA or PKG file, you need the right version of the **Xcode Archive** Step, or any other Step that is capable of building a binary file.

### Configuring the Step

Before you start using this Step, you need to do a couple of things:

* Register an app on the **My Apps** page of App Store Connect. Click on the **plus** sign and select the **New App** option. This requires an **admin** account.
* This Step requires an app signed with App Store Distibution provisioning profile. Make sure that you use the correct code signing files and the correct export method with the Step that builds your binary.
* Every build that you want to push to the App Store Connect must have a unique build and version number pair. Increment either or both before a new deploy to the App Store Connect.

To deploy your app with the Step:
1. Make sure that either the **IPA path** or the **PKG path** input has a valid value. The default value is perfect for most cases: it points to the output generated by the **Xcode Archive** Step.
1. Set up your connection depending on which authentication method you wish to use:
    - Use a previously configured Bitrise Apple Developer connection: Set the **Bitrise Apple Developer Connection** to `automatic` (this is the default setting), `api_key` or `apple_id`.
    - Provide manual Step inputs: Add authentication data depending on which authentication method you wish to use, either Apple ID or API key authentication. Set the **Bitrise Apple Developer Connection** to `off`. Use only one of the authentication methods.
        * For API key: Provide your **API Key: URL** (for example, https://URL/TO/AuthKey_something.p8 or file:///PATH/TO/AuthKey_something.p8) and the **API Key: Issuer ID** inputs.
        * For Apple ID: Use the Apple Developer connection based on Apple ID authentication. If no app-specific password has been added to the used connection, the **Apple ID: App-specific password** Step input will be used. Other authentication-related Step inputs are ignored.

### Troubleshooting

Use only one of the authentication methods, if you add both the Apple ID and the API key inputs the step will fail.

Make sure your Apple ID credentials are correct. Be aware that if you use two-factor authentication, you need to [set up](https://devcenter.bitrise.io/getting-started/configuring-bitrise-steps-that-require-apple-developer-account-data/#setting-up-connection-with-the-apple-id-and-password) a connection with Apple ID.

Always make sure that **Platform** input is set to the correct value.

The Step can also fail if the **Xcode Archive** Step - or any other Step that builds your binary - did not generate an IPA or PKG with a `app-store` export method.

### Useful links

- [Deploying an app to iTunesConnect](https://devcenter.bitrise.io/deploy/ios-deploy/deploying-an-ios-app-to-itunes-connect/)
- [iOS deployment](https://devcenter.bitrise.io/deploy/ios-deploy/ios-deploy-index/)

### Related Steps

- [Deploy to Google Play](https://www.bitrise.io/integrations/steps/google-play-deploy)
- [Xcode Archive & Export for iOS](https://www.bitrise.io/integrations/steps/xcode-archive)
- [Appetize.io deploy](https://www.bitrise.io/integrations/steps/appetize-deploy)
</details>

## 🧩 Get started

Add this step directly to your workflow in the [Bitrise Workflow Editor](https://devcenter.bitrise.io/steps-and-workflows/steps-and-workflows-index/).

You can also run this step directly with [Bitrise CLI](https://github.com/bitrise-io/bitrise).

## ⚙️ Configuration

<details>
<summary>Inputs</summary>

| Key | Description | Flags | Default |
| --- | --- | --- | --- |
| `connection` | The input determines the method used for Apple Service authentication. By default, any enabled Bitrise Apple Developer connection is used and other authentication-related Step inputs are ignored.  There are two types of Apple Developer connection you can enable on Bitrise: one is based on an API key of the App Store Connect API, the other is the Apple ID authentication. You can choose which type of Bitrise Apple Developer connection to use or you can tell the Step to only use Step inputs for authentication: - `automatic`: Use any enabled Apple Developer connection, either based on Apple ID authentication or API key authentication.  Step inputs are only used as a fallback. API key authentication has priority over Apple ID authentication in both cases. - `api_key`: Use the Apple Developer connection based on API key authentication. Authentication-related Step inputs are ignored. - `apple_id`: Use the Apple Developer connection based on Apple ID authentication and the **Application-specific password** Step input. Other authentication-related Step inputs are ignored. - `off`: Do not use any Apple Developer Connection. Only authentication-related Step inputs are considered. | required | `automatic` |
| `api_key_path` | Specify the path in an URL format where your API key is stored. For example: `https://URL/TO/AuthKey_[KEY_ID].p8` or `file:///PATH/TO/AuthKey_[KEY_ID].p8`. **NOTE:** The Step will only recognize the API key if the filename includes the  `KEY_ID` value as shown on the examples above.  You can upload your key on the **Generic File Storage** tab in the Workflow Editor and set the Environment Variable for the file here.  For example: `$BITRISEIO_MYKEY_URL` |  |  |
| `api_issuer` | Issuer ID. Required if **API Key: URL** (`api_key_path`) is specified. |  |  |
| `itunescon_user` | Email for Apple ID login. | sensitive |  |
| `password` | Password for the specified Apple ID. | sensitive |  |
| `app_password` | Use this input if TFA is enabled on the Apple ID but no app-specific password has been added to the used Bitrise Apple ID connection.  **NOTE:** Application-specific passwords can be created on the [AppleID Website](https://appleid.apple.com). It can be used to bypass two-factor authentication. | sensitive |  |
| `ipa_path` | Path to your IPA file to be deployed. **NOTE:** This input or `PKG path` is required. |  | `$BITRISE_IPA_PATH` |
| `pkg_path` | Path to your PKG file to be deployed. **NOTE:** This input or `IPA path` is required. |  | `$BITRISE_PKG_PATH` |
| `platform` | Specify the platform of the file. When `auto` is selected the step uses the `Info.plist` to set the platform. |  | `auto` |
| `retries` | Retry times when failed, set to `0` for infinite retry |  | `10` |
| `altool_options` | Options added to the end of the `altool` call. You can use multiple options, separated by a space character. Example: `--notarize-app --asc-provider" <<provider_id>>` |  |  |
</details>

<details>
<summary>Outputs</summary>
There are no outputs defined in this step
</details>

## 🙋 Contributing

We welcome [pull requests](https://github.com/bitrise-steplib/steps-deploy-to-itunesconnect-application-loader/pulls) and [issues](https://github.com/bitrise-steplib/steps-deploy-to-itunesconnect-application-loader/issues) against this repository.

For pull requests, work on your changes in a forked repository and use the Bitrise CLI to [run step tests locally](https://devcenter.bitrise.io/bitrise-cli/run-your-first-build/).

Learn more about developing steps:

- [Create your own step](https://devcenter.bitrise.io/contributors/create-your-own-step/)
- [Testing your Step](https://devcenter.bitrise.io/contributors/testing-and-versioning-your-steps/)
