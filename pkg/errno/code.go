package errno

var (
	// Common errors
	OK                  = &Errno{Code: 0, Message: "OK"}
	InternalServerError = &Errno{Code: 10001, Message: "Internal server error"}
	ErrBind             = &Errno{Code: 10002, Message: "Error occurred while binding the request body to the struct."}

	ErrValidation       = &Errno{Code: 20001, Message: "Validation failed."}
	ErrDatabase         = &Errno{Code: 20002, Message: "Database error."}
	ErrToken            = &Errno{Code: 20003, Message: "Error occurred while generating token."}
	ErrBadRequest       = &Errno{Code: 20004, Message: "Error occurred while payload is not bad."}
	ErrPermissionDenied = &Errno{Code: 20005, Message: "Permission denied."}

	// user errors
	ErrEncrypt              = &Errno{Code: 20101, Message: "Error occurred while encrypting the user password."}
	ErrUserNotFound         = &Errno{Code: 20102, Message: "The user was not found."}
	ErrTokenInvalid         = &Errno{Code: 20103, Message: "The token was invalid."}
	ErrPasswordIncorrect    = &Errno{Code: 20104, Message: "The password was incorrect."}
	ErrPasswordBase64Decode = &Errno{Code: 20105, Message: "The panic from password base64 string decoding."}

	// signup error
	ErrUserSignupEmailInvalid = &Errno{Code: 20201, Message: "The email from payload is invalid."}
	ErrUserExisted            = &Errno{Code: 20202, Message: "The user has existed."}

	// signin error
	ErrUserPasswordIncorrect = &Errno{Code: 20301, Message: "Password incorrect."}

	// mail error
	ErrMailSend             = &Errno{Code: 20401, Message: "Mail send failed."}
	ErrGenerateCaptchaToken = &Errno{Code: 20402, Message: "Generating captcha token failed."}

	// captcha error
	ErrUserVerifyFail = &Errno{Code: 20501, Message: "Verify captcha token failed."}

	// member profile error
	ErrMemberProfileNotFound = &Errno{Code: 20601, Message: "Member profile not found."}
	ErrMemberProfileExisted  = &Errno{Code: 20602, Message: "The user is already a muxi member."}
	ErrInvalidMemberGroup    = &Errno{Code: 20603, Message: "Member group is invalid."}
	ErrStudentIDExisted      = &Errno{Code: 20604, Message: "The student id is already used by another member."}

	// orm error
	ErrUserCreate = &Errno{Code: 30001, Message: "The (*UserModel)Create() method error."}
	ErrUserUpdate = &Errno{Code: 30002, Message: "The (*UserModel)Update() method error."}

	// oauth error
	ErrGenerateAuthCode                = &Errno{Code: 40001, Message: "Error occurred while generating auth code."}
	ErrGenerateAccessToken             = &Errno{Code: 40002, Message: "Error occurred while generating access token."}
	ErrRefreshToken                    = &Errno{Code: 40003, Message: "Error occurred while refreshing token."}
	ErrDomain                          = &Errno{Code: 40004, Message: "The domain is invalid."}
	ErrInvalidCASTicket                = &Errno{Code: 40005, Message: "The CAS ticket was invalid."}
	ErrOAuthClientRegistrationDisabled = &Errno{Code: 40006, Message: "OAuth client registration is temporarily disabled."}
	ErrOAuthClientCreate               = &Errno{Code: 40007, Message: "Error occurred while creating oauth client."}
	ErrNotMuxiMember                   = &Errno{Code: 40008, Message: "The account is not a muxi member."}
)
