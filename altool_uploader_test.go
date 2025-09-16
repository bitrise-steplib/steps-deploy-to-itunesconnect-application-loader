package main

import (
	"errors"
	"testing"

	"github.com/bitrise-io/go-utils/v2/log"
	"github.com/stretchr/testify/require"
)

func Test_parseJSONOutput(t *testing.T) {
	logger := log.NewLogger()
	tests := []struct {
		name          string
		stdOut        string
		want          altoolResult
		wantWarnings  []uploadError
		wantOutputErr error
		wantErr       bool
	}{
		{
			name: "Auth error",
			stdOut: `
{
  "os-version" : "Version 15.6.1 (Build 24G90)",
  "product-errors" : [
    {
      "code" : -19209,
      "message" : "Unable to authenticate.",
      "underlying-errors" : [

      ],
      "user-info" : {
        "NSLocalizedDescription" : "Unable to authenticate."
      }
    }
  ],
  "tool-path" : "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
  "tool-version" : "26.0.18 (170018)"
}
	`,
			want: altoolResult{
				OSVersion:   "Version 15.6.1 (Build 24G90)",
				ToolVersion: "26.0.18 (170018)",
				ToolPath:    "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
				ProductErrors: []productError{
					{
						Code:    -19209,
						Message: "Unable to authenticate.",
						UserInfo: userInfo{
							NSLocalizedDescription: "Unable to authenticate.",
						},
						UnderlyingErrors: []productError{},
					},
				},
			},
			wantOutputErr: uploadError{
				description: "Unable to authenticate.",
				errorCode:   -19209,
			},
			wantErr: false,
		},
		{
			name: "Successful upload",
			stdOut: `
{
  "details" : {
    "delivery-uuid" : "2d29ae8f-a628-4fee-bb75-d0fa4331d23c",
    "transferred" : "19555969 bytes in 1.831 seconds (10.7MB/s, 85.437Mbps)"
  },
  "os-version" : "Version 15.6.1 (Build 24G90)",
  "success-message" : "No errors uploading archive at '/var/folders/r5/gkvczn3j2tb0m79nwby9fjv80000gq/T/deploy2766821977/Application Loader Test.ipa'.",
  "tool-path" : "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
  "tool-version" : "26.0.18 (170018)"
}`,
			want: altoolResult{
				SuccessMessage: "No errors uploading archive at '/var/folders/r5/gkvczn3j2tb0m79nwby9fjv80000gq/T/deploy2766821977/Application Loader Test.ipa'.",
				SuccessDetails: successDetails{
					DeliveryUUID: "2d29ae8f-a628-4fee-bb75-d0fa4331d23c",
					Transferred:  "19555969 bytes in 1.831 seconds (10.7MB/s, 85.437Mbps)",
				},
				OSVersion:   "Version 15.6.1 (Build 24G90)",
				ToolVersion: "26.0.18 (170018)",
				ToolPath:    "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
			},
			wantOutputErr: nil,
		},
		{
			name: "1 product error, 1 underlying error",
			stdOut: `
{
	"os-version" : "Version 15.6.1 (Build 24G90)",
	"product-errors" : [
		{
			"code" : 409,
			"message" : "Validation failed",
			"underlying-errors" : [
				{
					"code" : -19241,
					"message" : "Validation failed",
					"underlying-errors" : [

					],
					"user-info" : {
						"NSLocalizedDescription" : "Validation failed",
						"NSLocalizedFailureReason" : "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again.",
						"code" : "STATE_ERROR.VALIDATION_ERROR",
						"detail" : "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again.",
						"id" : "b753c995-ba50-4213-a173-fe74e14f0b48",
						"status" : "409",
						"title" : "Validation failed"
					}
				}
			],
			"user-info" : {
				"NSLocalizedDescription" : "Validation failed",
				"NSLocalizedFailureReason" : "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again. (ID: b753c995-ba50-4213-a173-fe74e14f0b48)",
				"NSUnderlyingError" : "Error Domain=IrisAPI Code=-19241 \"Validation failed\" UserInfo={status=409, detail=Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again., id=b753c995-ba50-4213-a173-fe74e14f0b48, code=STATE_ERROR.VALIDATION_ERROR, title=Validation failed, NSLocalizedFailureReason=Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again., NSLocalizedDescription=Validation failed}",
				"iris-code" : "STATE_ERROR.VALIDATION_ERROR"
			}
		}
	],
	"tool-path" : "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
	"tool-version" : "26.0.18 (170018)"
}
				`,
			want: altoolResult{
				OSVersion:   "Version 15.6.1 (Build 24G90)",
				ToolVersion: "26.0.18 (170018)",
				ToolPath:    "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
				ProductErrors: []productError{
					{
						Code:    409,
						Message: "Validation failed",
						UserInfo: userInfo{
							NSLocalizedDescription:   "Validation failed",
							NSLocalizedFailureReason: "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again. (ID: b753c995-ba50-4213-a173-fe74e14f0b48)",
							NSUnderlyingError:        "Error Domain=IrisAPI Code=-19241 \"Validation failed\" UserInfo={status=409, detail=Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again., id=b753c995-ba50-4213-a173-fe74e14f0b48, code=STATE_ERROR.VALIDATION_ERROR, title=Validation failed, NSLocalizedFailureReason=Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again., NSLocalizedDescription=Validation failed}",
							IrisCode:                 "STATE_ERROR.VALIDATION_ERROR",
						},
						UnderlyingErrors: []productError{
							{
								Code:    -19241,
								Message: "Validation failed",
								UserInfo: userInfo{
									NSLocalizedDescription:   "Validation failed",
									NSLocalizedFailureReason: "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again.",
									Code:                     "STATE_ERROR.VALIDATION_ERROR",
									Detail:                   "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again.",
									ID:                       "b753c995-ba50-4213-a173-fe74e14f0b48",
									Status:                   "409",
									Title:                    "Validation failed",
								},
								UnderlyingErrors: []productError{},
							},
						},
					},
				},
			},
			wantOutputErr: uploadError{
				description: "Validation failed",
				reason:      "Upload limit reached. The upload limit for your application has been reached. Please wait 1 day and try again. (ID: b753c995-ba50-4213-a173-fe74e14f0b48)",
				errorCode:   409,
				errorID:     "STATE_ERROR.VALIDATION_ERROR",
			},
			wantErr: false,
		},
		{
			name: "Success with warnings",
			stdOut: `
{
  "details" : {
    "delivery-uuid" : "6eb04796-467f-4e58-87c1-97afae95ce8e",
    "transferred" : "19555997 bytes in 1.532 seconds (12.8MB/s, 102.106Mbps)"
  },
  "os-version" : "Version 15.6.1 (Build 24G90)",
  "success-message" : "No errors, 2 warnings, uploading archive at '/var/folders/r5/gkvczn3j2tb0m79nwby9fjv80000gq/T/deploy3195197989/Application Loader Test.ipa'.",
  "tool-path" : "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
  "tool-version" : "26.0.18 (170018)",
  "warnings" : [
    {
      "code" : -19237,
      "message" : "A non-validation error occurred during validation.",
      "underlying-errors" : [
        {
          "code" : -19237,
          "message" : "The server returned unexpected content.",
          "underlying-errors" : [

          ],
          "user-info" : {
            "NSLocalizedDescription" : "The server returned unexpected content.",
            "NSLocalizedFailureReason" : "Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n"
          }
        }
      ],
      "user-info" : {
        "NSLocalizedDescription" : "A non-validation error occurred during validation.",
        "NSLocalizedFailureReason" : "Skipping validation.",
        "NSUnderlyingError" : "Error Domain=ITunesConnectFoundationErrorDomain Code=-19237 \"The server returned unexpected content.\" UserInfo={NSLocalizedDescription=The server returned unexpected content., NSLocalizedFailureReason=Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n}"
      }
    },
    {
      "code" : -19237,
      "message" : "The server returned unexpected content.",
      "underlying-errors" : [

      ],
      "user-info" : {
        "NSLocalizedDescription" : "The server returned unexpected content.",
        "NSLocalizedFailureReason" : "Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n"
      }
    }
  ]
}`,
			want: altoolResult{
				SuccessMessage: "No errors, 2 warnings, uploading archive at '/var/folders/r5/gkvczn3j2tb0m79nwby9fjv80000gq/T/deploy3195197989/Application Loader Test.ipa'.",
				SuccessDetails: successDetails{
					DeliveryUUID: "6eb04796-467f-4e58-87c1-97afae95ce8e",
					Transferred:  "19555997 bytes in 1.532 seconds (12.8MB/s, 102.106Mbps)",
				},
				OSVersion:   "Version 15.6.1 (Build 24G90)",
				ToolVersion: "26.0.18 (170018)",
				ToolPath:    "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
				Warnings: []productError{
					{
						Code:    -19237,
						Message: "A non-validation error occurred during validation.",
						UserInfo: userInfo{
							NSLocalizedDescription:   "A non-validation error occurred during validation.",
							NSLocalizedFailureReason: "Skipping validation.",
							NSUnderlyingError:        "Error Domain=ITunesConnectFoundationErrorDomain Code=-19237 \"The server returned unexpected content.\" UserInfo={NSLocalizedDescription=The server returned unexpected content., NSLocalizedFailureReason=Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n}",
						},
						UnderlyingErrors: []productError{
							{
								Code:    -19237,
								Message: "The server returned unexpected content.",
								UserInfo: userInfo{
									NSLocalizedDescription:   "The server returned unexpected content.",
									NSLocalizedFailureReason: "Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n",
								},
								UnderlyingErrors: []productError{},
							},
						},
					},
					{
						Code:    -19237,
						Message: "The server returned unexpected content.",
						UserInfo: userInfo{
							NSLocalizedDescription:   "The server returned unexpected content.",
							NSLocalizedFailureReason: "Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n",
						},
						UnderlyingErrors: []productError{},
					},
				},
			},
			wantWarnings: []uploadError{
				{
					description: "A non-validation error occurred during validation.",
					reason:      "Skipping validation.",
					errorCode:   -19237,
				},
				{
					description: "The server returned unexpected content.",
					reason:      "Internal Server Error\n\nRequest ID: NWCBL6W4YYM6MOFQFOQTQANWOU.0.0\n",
					errorCode:   -19237,
				},
			},
			wantOutputErr: nil,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseJSONOutput(logger, tt.stdOut)
			if tt.wantErr {
				require.Error(t, gotErr)
			}

			require.NoError(t, gotErr)
			require.Equal(t, tt.want, got)

			if len(tt.wantWarnings) > 0 {
				gotWarnings := got.getWarnings()
				require.ElementsMatch(t, tt.wantWarnings, gotWarnings)
			} else {
				require.Empty(t, got.getWarnings())
			}

			if tt.wantOutputErr == nil {
				require.NoError(t, got.getError())
			} else {
				var gotUploadErr uploadError
				isExpectedErr := errors.As(got.getError(), &gotUploadErr)
				require.True(t, isExpectedErr)
				require.Equal(t, tt.wantOutputErr, gotUploadErr)
			}
		})
	}
}

func Test_parseAltoolOutput(t *testing.T) {
	tests := []struct {
		name        string
		logger      log.Logger
		ioString    string
		errorString string
		isJson      bool
		want        *altoolResult
		wantErr     bool
	}{
		/*{
					name:   "Fallback to non-JSON output, Internal Server Error",
					logger: log.NewLogger(),
					errorString: `Uploading IPA failed: Upload failed, output: Running altool at path '/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources/altool'...

		2025-09-15 12:55:08.803 ERROR: [ContentDelivery.Uploader.13AF057C0] CREATE BUILD (ASSET_UPLOAD): received status code 500; internal server error. (QRVCH6RCR4HT4P5YNGI3PYVPSQ) (500) Internal Server Error

		Request ID: QRVCH6RCR4HT4P5YNGI3PYVPSQ.0.0
		2025-09-15 12:55:09.216 ERROR: [ContentDelivery.Uploader.13AF057C0] The server returned unexpected content. (-19237) Internal Server Error

		Request ID: QRVCH6RCR4HT4P5YNGI3PYVPSQ.0.0
		2025-09-15 12:55:09.218 ERROR: [altool.13AF057C0] Failed to upload package.

		Uploading IPA failed: Upload failed, output: Running altool at path '/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources/altool'...

		2025-09-15 12:55:08.803 ERROR: [ContentDelivery.Uploader.13AF057C0] CREATE BUILD (ASSET_UPLOAD): received status code 500; internal server error. (QRVCH6RCR4HT4P5YNGI3PYVPSQ) (500) Internal Server Error

		Request ID: QRVCH6RCR4HT4P5YNGI3PYVPSQ.0.0
		2025-09-15 12:55:09.216 ERROR: [ContentDelivery.Uploader.13AF057C0] The server returned unexpected content. (-19237) Internal Server Error

		Request ID: QRVCH6RCR4HT4P5YNGI3PYVPSQ.0.0
		2025-09-15 12:55:09.218 ERROR: [altool.13AF057C0] Failed to upload package.`,
					isJson:  true,
					want:    nil,
					wantErr: true,
				},
				{
					name:   "Fallback to non-JSON output, auth error",
					logger: log.NewLogger(),
					errorString: `
		[stderr] Running altool at path '/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources/altool'...
		[stderr]
		[stderr] 2025-09-16 15:37:02.262 ERROR: [altool.14AF0D200] GET APP SETTINGS: failed to authenticate. The server response was: {
		[stderr]     errors =     (
		[stderr]                 {
		[stderr]             code = "NOT_AUTHORIZED";
		[stderr]             detail = "Provide a properly configured and signed bearer token, and make sure that it has not expired. Learn more about Generating Tokens for API Requests https://developer.apple.com/go/?id=api-generating-tokens";
		[stderr]             status = 401;
		[stderr]             title = "Authentication credentials are missing or invalid.";
		[stderr]         }
		[stderr]     );
		[stderr] }
		[stderr] 2025-09-16 15:37:02.262 ERROR: [altool.14AF0D200] GET APP SETTINGS: received status code 401, auth issue.
		[stderr] 2025-09-16 15:37:03.040 ERROR: [CDASCAPI.14AF0D200] APP STORE CONNECT API list-apps: failed to authenticate. The server response was: {
		[stderr]     errors =     (
		[stderr]                 {
		[stderr]             code = "NOT_AUTHORIZED";
		[stderr]             detail = "Provide a properly configured and signed bearer token, and make sure that it has not expired. Learn more about Generating Tokens for API Requests https://developer.apple.com/go/?id=api-generating-tokens";
		[stderr]             status = 401;
		[stderr]             title = "Authentication credentials are missing or invalid.";
		[stderr]         }
		[stderr]     );
		[stderr] }
		[stderr] 2025-09-16 15:37:03.040 ERROR: [CDASCAPI.14AF0D200] APP STORE CONNECT API list-apps: received status code 401, auth issue.
		[stderr] 2025-09-16 15:37:03.041 ERROR: [altool.14AF0D200] Failed to determine the Apple ID from Bundle ID 'com.bitrise.Application-Loader-Test' with platform 'IOS'. Unable to authenticate. (-19209) (12)
		[stderr]`,
					isJson:  true,
					want:    nil,
					wantErr: true,
				},*/
		{
			name:   "JSON output",
			logger: log.NewLogger(),
			ioString: `{
  "os-version" : "Version 15.6.1 (Build 24G90)",
  "product-errors" : [
    {
      "code" : -19232,
      "message" : "The provided entity includes an attribute with a value that has already been used",
      "underlying-errors" : [
        {
          "code" : -19241,
          "message" : "The provided entity includes an attribute with a value that has already been used",
          "underlying-errors" : [

          ],
          "user-info" : {
            "NSLocalizedDescription" : "The provided entity includes an attribute with a value that has already been used",
            "NSLocalizedFailureReason" : "The bundle version must be higher than the previously uploaded version.",
            "code" : "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
            "detail" : "The bundle version must be higher than the previously uploaded version.",
            "id" : "a6a0f65a-22ee-4529-8249-e0df8bc254dc",
            "meta" : "{\n    previousBundleVersion = 2509152209882287;\n}",
            "source" : "{\n    pointer = \"/data/attributes/cfBundleVersion\";\n}",
            "status" : "409",
            "title" : "The provided entity includes an attribute with a value that has already been used"
          }
        }
      ],
      "user-info" : {
        "NSLocalizedDescription" : "The provided entity includes an attribute with a value that has already been used",
        "NSLocalizedFailureReason" : "The bundle version must be higher than the previously uploaded version: ‘2509152209882287’. (ID: a6a0f65a-22ee-4529-8249-e0df8bc254dc)",
        "NSUnderlyingError" : "Error Domain=IrisAPI Code=-19241 \"The provided entity includes an attribute with a value that has already been used\" UserInfo={status=409, detail=The bundle version must be higher than the previously uploaded version., source={\n    pointer = \"/data/attributes/cfBundleVersion\";\n}, id=a6a0f65a-22ee-4529-8249-e0df8bc254dc, code=ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE, title=The provided entity includes an attribute with a value that has already been used, meta={\n    previousBundleVersion = 2509152209882287;\n}, NSLocalizedDescription=The provided entity includes an attribute with a value that has already been used, NSLocalizedFailureReason=The bundle version must be higher than the previously uploaded version.}",
        "iris-code" : "ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
        "previousBundleVersion" : "2509152209882287"
      }
    }
  ],
  "tool-path" : "/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources",
  "tool-version" : "26.0.18 (170018)"
}`,
			errorString: `
Running altool at path '/Applications/Xcode26RC.app/Contents/SharedFrameworks/ContentDelivery.framework/Resources/altool'...

Contacting Apple Services…
2025-09-16 17:21:44.353 ERROR: [ContentDelivery.Uploader.14C70D4C0] The provided entity includes an attribute with a value that has already been used (-19232) The bundle version must be higher than the previously uploaded version: ‘2509152209882287’. (ID: a6a0f65a-22ee-4529-8249-e0df8bc254dc)
   NSUnderlyingError : The provided entity includes an attribute with a value that has already been used (-19241) The bundle version must be higher than the previously uploaded version.
      status : 409
      detail : The bundle version must be higher than the previously uploaded version.
      source : 
         pointer : /data/attributes/cfBundleVersion
      id : a6a0f65a-22ee-4529-8249-e0df8bc254dc
      code : ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE
      title : The provided entity includes an attribute with a value that has already been used
      meta : 
         previousBundleVersion : 2509152209882287
   previousBundleVersion : 2509152209882287
   iris-code : ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE
2025-09-16 17:21:44.355 ERROR: [altool.14C70D4C0] Failed to upload package.`,
			isJson:  true,
			wantErr: true,
			want:    &altoolResult{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := parseAltoolOutput(tt.logger, tt.ioString, tt.errorString, tt.isJson)
			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tt.want, got)
		})
	}
}
