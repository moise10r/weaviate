//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2024 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

// Code generated by go-swagger; DO NOT EDIT.

package authz

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"

	"github.com/weaviate/weaviate/entities/models"
)

// DeleteRoleNoContentCode is the HTTP code returned for type DeleteRoleNoContent
const DeleteRoleNoContentCode int = 204

/*
DeleteRoleNoContent Successfully deleted.

swagger:response deleteRoleNoContent
*/
type DeleteRoleNoContent struct {
}

// NewDeleteRoleNoContent creates DeleteRoleNoContent with default headers values
func NewDeleteRoleNoContent() *DeleteRoleNoContent {

	return &DeleteRoleNoContent{}
}

// WriteResponse to the client
func (o *DeleteRoleNoContent) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(204)
}

// DeleteRoleBadRequestCode is the HTTP code returned for type DeleteRoleBadRequest
const DeleteRoleBadRequestCode int = 400

/*
DeleteRoleBadRequest Bad request

swagger:response deleteRoleBadRequest
*/
type DeleteRoleBadRequest struct {

	/*
	  In: Body
	*/
	Payload *models.ErrorResponse `json:"body,omitempty"`
}

// NewDeleteRoleBadRequest creates DeleteRoleBadRequest with default headers values
func NewDeleteRoleBadRequest() *DeleteRoleBadRequest {

	return &DeleteRoleBadRequest{}
}

// WithPayload adds the payload to the delete role bad request response
func (o *DeleteRoleBadRequest) WithPayload(payload *models.ErrorResponse) *DeleteRoleBadRequest {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the delete role bad request response
func (o *DeleteRoleBadRequest) SetPayload(payload *models.ErrorResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *DeleteRoleBadRequest) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(400)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// DeleteRoleUnauthorizedCode is the HTTP code returned for type DeleteRoleUnauthorized
const DeleteRoleUnauthorizedCode int = 401

/*
DeleteRoleUnauthorized Unauthorized or invalid credentials.

swagger:response deleteRoleUnauthorized
*/
type DeleteRoleUnauthorized struct {
}

// NewDeleteRoleUnauthorized creates DeleteRoleUnauthorized with default headers values
func NewDeleteRoleUnauthorized() *DeleteRoleUnauthorized {

	return &DeleteRoleUnauthorized{}
}

// WriteResponse to the client
func (o *DeleteRoleUnauthorized) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(401)
}

// DeleteRoleForbiddenCode is the HTTP code returned for type DeleteRoleForbidden
const DeleteRoleForbiddenCode int = 403

/*
DeleteRoleForbidden Forbidden

swagger:response deleteRoleForbidden
*/
type DeleteRoleForbidden struct {

	/*
	  In: Body
	*/
	Payload *models.ErrorResponse `json:"body,omitempty"`
}

// NewDeleteRoleForbidden creates DeleteRoleForbidden with default headers values
func NewDeleteRoleForbidden() *DeleteRoleForbidden {

	return &DeleteRoleForbidden{}
}

// WithPayload adds the payload to the delete role forbidden response
func (o *DeleteRoleForbidden) WithPayload(payload *models.ErrorResponse) *DeleteRoleForbidden {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the delete role forbidden response
func (o *DeleteRoleForbidden) SetPayload(payload *models.ErrorResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *DeleteRoleForbidden) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(403)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}

// DeleteRoleInternalServerErrorCode is the HTTP code returned for type DeleteRoleInternalServerError
const DeleteRoleInternalServerErrorCode int = 500

/*
DeleteRoleInternalServerError An error has occurred while trying to fulfill the request. Most likely the ErrorResponse will contain more information about the error.

swagger:response deleteRoleInternalServerError
*/
type DeleteRoleInternalServerError struct {

	/*
	  In: Body
	*/
	Payload *models.ErrorResponse `json:"body,omitempty"`
}

// NewDeleteRoleInternalServerError creates DeleteRoleInternalServerError with default headers values
func NewDeleteRoleInternalServerError() *DeleteRoleInternalServerError {

	return &DeleteRoleInternalServerError{}
}

// WithPayload adds the payload to the delete role internal server error response
func (o *DeleteRoleInternalServerError) WithPayload(payload *models.ErrorResponse) *DeleteRoleInternalServerError {
	o.Payload = payload
	return o
}

// SetPayload sets the payload to the delete role internal server error response
func (o *DeleteRoleInternalServerError) SetPayload(payload *models.ErrorResponse) {
	o.Payload = payload
}

// WriteResponse to the client
func (o *DeleteRoleInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.WriteHeader(500)
	if o.Payload != nil {
		payload := o.Payload
		if err := producer.Produce(rw, payload); err != nil {
			panic(err) // let the recovery middleware deal with this
		}
	}
}