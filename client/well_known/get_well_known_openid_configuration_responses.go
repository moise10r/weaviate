//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2020 SeMI Holding B.V. (registered @ Dutch Chamber of Commerce no 75221632). All rights reserved.
//  LICENSE WEAVIATE OPEN SOURCE: https://www.semi.technology/playbook/playbook/contract-weaviate-OSS.html
//  LICENSE WEAVIATE ENTERPRISE: https://www.semi.technology/playbook/contract-weaviate-enterprise.html
//  CONCEPT: Bob van Luijt (@bobvanluijt)
//  CONTACT: hello@semi.technology
//

// Code generated by go-swagger; DO NOT EDIT.

package well_known

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// GetWellKnownOpenidConfigurationReader is a Reader for the GetWellKnownOpenidConfiguration structure.
type GetWellKnownOpenidConfigurationReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetWellKnownOpenidConfigurationReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetWellKnownOpenidConfigurationOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 404:
		result := NewGetWellKnownOpenidConfigurationNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewGetWellKnownOpenidConfigurationOK creates a GetWellKnownOpenidConfigurationOK with default headers values
func NewGetWellKnownOpenidConfigurationOK() *GetWellKnownOpenidConfigurationOK {
	return &GetWellKnownOpenidConfigurationOK{}
}

/*GetWellKnownOpenidConfigurationOK handles this case with default header values.

Successful response, inspect body
*/
type GetWellKnownOpenidConfigurationOK struct {
	Payload *GetWellKnownOpenidConfigurationOKBody
}

func (o *GetWellKnownOpenidConfigurationOK) Error() string {
	return fmt.Sprintf("[GET /.well-known/openid-configuration][%d] getWellKnownOpenidConfigurationOK  %+v", 200, o.Payload)
}

func (o *GetWellKnownOpenidConfigurationOK) GetPayload() *GetWellKnownOpenidConfigurationOKBody {
	return o.Payload
}

func (o *GetWellKnownOpenidConfigurationOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(GetWellKnownOpenidConfigurationOKBody)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewGetWellKnownOpenidConfigurationNotFound creates a GetWellKnownOpenidConfigurationNotFound with default headers values
func NewGetWellKnownOpenidConfigurationNotFound() *GetWellKnownOpenidConfigurationNotFound {
	return &GetWellKnownOpenidConfigurationNotFound{}
}

/*GetWellKnownOpenidConfigurationNotFound handles this case with default header values.

Not found, no oidc provider present
*/
type GetWellKnownOpenidConfigurationNotFound struct {
}

func (o *GetWellKnownOpenidConfigurationNotFound) Error() string {
	return fmt.Sprintf("[GET /.well-known/openid-configuration][%d] getWellKnownOpenidConfigurationNotFound ", 404)
}

func (o *GetWellKnownOpenidConfigurationNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

/*GetWellKnownOpenidConfigurationOKBody get well known openid configuration o k body
swagger:model GetWellKnownOpenidConfigurationOKBody
*/
type GetWellKnownOpenidConfigurationOKBody struct {

	// OAuth Client ID
	ClientID string `json:"clientId,omitempty"`

	// The Location to redirect to
	Href string `json:"href,omitempty"`
}

// Validate validates this get well known openid configuration o k body
func (o *GetWellKnownOpenidConfigurationOKBody) Validate(formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (o *GetWellKnownOpenidConfigurationOKBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *GetWellKnownOpenidConfigurationOKBody) UnmarshalBinary(b []byte) error {
	var res GetWellKnownOpenidConfigurationOKBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}