/*                          _       _
 *__      _____  __ ___   ___  __ _| |_ ___
 *\ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
 * \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
 *  \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
 *
 * Copyright © 2016 - 2019 Weaviate. All rights reserved.
 * LICENSE: https://github.com/creativesoftwarefdn/weaviate/blob/develop/LICENSE.md
 * DESIGN: Bob van Luijt (bob@k10y.co)
 */

// Package restapi with all rest API functions.
package restapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creativesoftwarefdn/weaviate/restapi/operations/graphql"
	"github.com/creativesoftwarefdn/weaviate/restapi/operations/knowledge_tools"
	"github.com/creativesoftwarefdn/weaviate/restapi/operations/meta"
	"github.com/creativesoftwarefdn/weaviate/restapi/operations/p2_p"

	errors "github.com/go-openapi/errors"
	runtime "github.com/go-openapi/runtime"
	middleware "github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/rs/cors"
	"google.golang.org/grpc/grpclog"

	"github.com/creativesoftwarefdn/weaviate/auth"
	weaviateBroker "github.com/creativesoftwarefdn/weaviate/broker"
	"github.com/creativesoftwarefdn/weaviate/config"
	"github.com/creativesoftwarefdn/weaviate/database"
	dbconnector "github.com/creativesoftwarefdn/weaviate/database/connectors"
	dblisting "github.com/creativesoftwarefdn/weaviate/database/listing"
	connutils "github.com/creativesoftwarefdn/weaviate/database/utils"
	"github.com/creativesoftwarefdn/weaviate/database/schema"
	db_local_schema_manager "github.com/creativesoftwarefdn/weaviate/database/schema_manager/local"
	"github.com/creativesoftwarefdn/weaviate/graphqlapi"
	graphqlnetwork "github.com/creativesoftwarefdn/weaviate/graphqlapi/network"
	"github.com/creativesoftwarefdn/weaviate/lib/delayed_unlock"
	"github.com/creativesoftwarefdn/weaviate/messages"
	"github.com/creativesoftwarefdn/weaviate/models"
	"github.com/creativesoftwarefdn/weaviate/restapi/operations"
	rest_api_utils "github.com/creativesoftwarefdn/weaviate/restapi/rest_api_utils"
	"github.com/creativesoftwarefdn/weaviate/validation"

	libcontextionary "github.com/creativesoftwarefdn/weaviate/contextionary"
	"github.com/creativesoftwarefdn/weaviate/graphqlapi/graphiql"
	"github.com/creativesoftwarefdn/weaviate/lib/feature_flags"
	libnetwork "github.com/creativesoftwarefdn/weaviate/network"
	"github.com/creativesoftwarefdn/weaviate/network/common/peers"
	libnetworkFake "github.com/creativesoftwarefdn/weaviate/network/fake"
	libnetworkP2P "github.com/creativesoftwarefdn/weaviate/network/p2p"
	"github.com/creativesoftwarefdn/weaviate/restapi/swagger_middleware"
)

const pageOverride int = 1
const error422 string = "The request is well-formed but was unable to be followed due to semantic errors."

var connectorOptionGroup *swag.CommandLineOptionsGroup
var contextionary libcontextionary.Contextionary
var network libnetwork.Network
var serverConfig *config.WeaviateConfig
var graphQL graphqlapi.GraphQL
var messaging *messages.Messaging

var db database.Database

type dbAndNetwork struct {
	database.Database
	libnetwork.Network
}

func (d dbAndNetwork) GetNetworkResolver() graphqlnetwork.Resolver {
	return d.Network
}

type keyTokenHeader struct {
	Key   strfmt.UUID `json:"key"`
	Token strfmt.UUID `json:"token"`
}

func init() {
	discard := ioutil.Discard
	myGRPCLogger := log.New(discard, "", log.LstdFlags)
	grpclog.SetLogger(myGRPCLogger)

	// Create temp folder if it does not exist
	tempFolder := "temp"
	if _, err := os.Stat(tempFolder); os.IsNotExist(err) {
		messaging.InfoMessage("Temp folder created...")
		os.Mkdir(tempFolder, 0766)
	}
}

// getLimit returns the maximized limit
func getLimit(paramMaxResults *int64) int {
	maxResults := serverConfig.Environment.Limit
	// Get the max results from params, if exists
	if paramMaxResults != nil {
		maxResults = *paramMaxResults
	}

	// Max results form URL, otherwise max = config.Limit.
	return int(math.Min(float64(maxResults), float64(serverConfig.Environment.Limit)))
}

// getPage returns the page if set
func getPage(paramPage *int64) int {
	page := int64(pageOverride)
	// Get the page from params, if exists
	if paramPage != nil {
		page = *paramPage
	}

	// Page form URL, otherwise max = config.Limit.
	return int(page)
}

func generateMultipleRefObject(keyIDs []strfmt.UUID) models.MultipleRef {
	// Init the response
	refs := models.MultipleRef{}

	// Init localhost
	url := serverConfig.GetHostAddress()

	// Generate SingleRefs
	for _, keyID := range keyIDs {
		refs = append(refs, &models.SingleRef{
			LocationURL:  &url,
			NrDollarCref: keyID,
			Type:         string(connutils.RefTypeKey),
		})
	}

	return refs
}

func deleteKey(ctx context.Context, databaseConnector dbconnector.DatabaseConnector, parentUUID strfmt.UUID) {
	// Find its children
	var allIDs []strfmt.UUID

	// Get all the children to remove
	allIDs, _ = auth.GetKeyChildrenUUIDs(ctx, databaseConnector, parentUUID, false, allIDs, 0, 0)

	// Append the children to the parent UUIDs to remove all
	allIDs = append(allIDs, parentUUID)

	// Delete for every child
	for _, keyID := range allIDs {
		// Initialize response
		keyResponse := models.KeyGetResponse{}

		// Get the key to delete
		databaseConnector.GetKey(ctx, keyID, &keyResponse)

		databaseConnector.DeleteKey(ctx, &keyResponse.Key, keyID)
	}
}

func configureFlags(api *operations.WeaviateAPI) {
	connectorOptionGroup = config.GetConfigOptionGroup()

	api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{
		*connectorOptionGroup,
	}
}

// createErrorResponseObject is a common function to create an error response
func createErrorResponseObject(messages ...string) *models.ErrorResponse {
	// Initialize return value
	er := &models.ErrorResponse{}

	// appends all error messages to the error
	for _, message := range messages {
		er.Error = append(er.Error, &models.ErrorResponseErrorItems0{
			Message: message,
		})
	}

	return er
}

func headerAPIKeyHandling(ctx context.Context, keyToken string) (*models.KeyTokenGetResponse, error) {
	dbLock := db.ConnectorLock()
	defer dbLock.Unlock()
	dbConnector := dbLock.Connector()

	// Convert JSON string to struct
	kth := keyTokenHeader{}
	json.Unmarshal([]byte(keyToken), &kth)

	// Validate both headers
	if kth.Key == "" || kth.Token == "" {
		return nil, errors.New(401, connutils.StaticMissingHeader)
	}

	// Create key
	validatedKey := models.KeyGetResponse{}

	// Check if the user has access, true if yes
	hashed, err := dbConnector.ValidateToken(ctx, kth.Key, &validatedKey)

	// Error printing
	if err != nil {
		return nil, errors.New(401, err.Error())
	}

	// Check token
	if !connutils.TokenHashCompare(hashed, kth.Token) {
		return nil, errors.New(401, connutils.StaticInvalidToken)
	}

	// Validate the key on expiry time
	currentUnix := connutils.NowUnix()

	if validatedKey.KeyExpiresUnix != -1 && validatedKey.KeyExpiresUnix < currentUnix {
		return nil, errors.New(401, connutils.StaticKeyExpired)
	}

	// Init response object
	validatedKeyToken := models.KeyTokenGetResponse{}
	validatedKeyToken.KeyGetResponse = validatedKey
	validatedKeyToken.Token = kth.Token

	// key is valid, next step is allowing per Handler handling
	return &validatedKeyToken, nil
}

func configureAPI(api *operations.WeaviateAPI) http.Handler {
	api.ServeError = errors.ServeError

	api.JSONConsumer = runtime.JSONConsumer()

	setupSchemaHandlers(api)
	setupThingsHandlers(api)
	setupActionsHandlers(api)
	setupKeysHandlers(api)

	/*
	 * HANDLE X-API-KEY
	 */
	// Applies when the "X-API-KEY" header is set
	api.APIKeyAuth = func(token string) (interface{}, error) {
		ctx := context.Background()
		return headerAPIKeyHandling(ctx, token)
	}

	/*
	 * HANDLE X-API-TOKEN
	 */
	// Applies when the "X-API-TOKEN" header is set
	api.APITokenAuth = func(token string) (interface{}, error) {
		ctx := context.Background()
		return headerAPIKeyHandling(ctx, token)
	}

	/*
	 * HANDLE KEYS
	 */

	api.MetaWeaviateMetaGetHandler = meta.WeaviateMetaGetHandlerFunc(func(params meta.WeaviateMetaGetParams, principal interface{}) middleware.Responder {
		dbLock := db.ConnectorLock()
		defer dbLock.Unlock()
		databaseSchema := schema.HackFromDatabaseSchema(dbLock.GetSchema())
		// Create response object
		metaResponse := &models.Meta{}

		// Set the response object's values
		metaResponse.Hostname = serverConfig.GetHostAddress()
		metaResponse.ActionsSchema = databaseSchema.ActionSchema.Schema
		metaResponse.ThingsSchema = databaseSchema.ThingSchema.Schema

		return meta.NewWeaviateMetaGetOK().WithPayload(metaResponse)
	})

	api.P2PWeaviateP2pGenesisUpdateHandler = p2_p.WeaviateP2pGenesisUpdateHandlerFunc(func(params p2_p.WeaviateP2pGenesisUpdateParams) middleware.Responder {
		newPeers := make([]peers.Peer, 0)

		for _, genesisPeer := range params.Peers {
			peer := peers.Peer{
				ID:         genesisPeer.ID,
				Name:       genesisPeer.Name,
				URI:        genesisPeer.URI,
				SchemaHash: genesisPeer.SchemaHash,
			}

			newPeers = append(newPeers, peer)
		}

		err := network.UpdatePeers(newPeers)

		if err == nil {
			return p2_p.NewWeaviateP2pGenesisUpdateOK()
		}
		return p2_p.NewWeaviateP2pGenesisUpdateInternalServerError()
	})

	api.P2PWeaviateP2pHealthHandler = p2_p.WeaviateP2pHealthHandlerFunc(func(params p2_p.WeaviateP2pHealthParams) middleware.Responder {
		// For now, always just return success.
		return middleware.NotImplemented("operation P2PWeaviateP2pHealth has not yet been implemented")
	})

	api.GraphqlWeaviateGraphqlPostHandler = graphql.WeaviateGraphqlPostHandlerFunc(func(params graphql.WeaviateGraphqlPostParams, principal interface{}) middleware.Responder {
		defer messaging.TimeTrack(time.Now())
		messaging.DebugMessage("Starting GraphQL resolving")

		errorResponse := &models.ErrorResponse{}

		// Get all input from the body of the request, as it is a POST.
		query := params.Body.Query
		operationName := params.Body.OperationName

		// If query is empty, the request is unprocessable
		if query == "" {
			errorResponse.Error = []*models.ErrorResponseErrorItems0{
				&models.ErrorResponseErrorItems0{
					Message: "query cannot be empty",
				}}
			return graphql.NewWeaviateGraphqlPostUnprocessableEntity().WithPayload(errorResponse)
		}

		// Only set variables if exists in request
		var variables map[string]interface{}
		if params.Body.Variables != nil {
			variables = params.Body.Variables.(map[string]interface{})
		}

		// Add security principal to context that we pass on to the GraphQL resolver.
		graphql_context := context.WithValue(params.HTTPRequest.Context(), "principal", (principal.(*models.KeyTokenGetResponse)))

		if graphQL == nil {
			errorResponse.Error = []*models.ErrorResponseErrorItems0{
				&models.ErrorResponseErrorItems0{
					Message: "no graphql provider present, " +
						"this is most likely because no schema is present. Import a schema first!",
				}}
			return graphql.NewWeaviateGraphqlPostUnprocessableEntity().WithPayload(errorResponse)
		}

		result := graphQL.Resolve(query, operationName, variables, graphql_context)

		// Marshal the JSON
		resultJSON, jsonErr := json.Marshal(result)
		if jsonErr != nil {
			errorResponse.Error = []*models.ErrorResponseErrorItems0{
				&models.ErrorResponseErrorItems0{
					Message: fmt.Sprintf("couldn't marshal json: %s", jsonErr),
				}}
			return graphql.NewWeaviateGraphqlPostUnprocessableEntity().WithPayload(errorResponse)
		}

		// Put the data in a response ready object
		graphQLResponse := &models.GraphQLResponse{}
		marshallErr := json.Unmarshal(resultJSON, graphQLResponse)

		// If json gave error, return nothing.
		if marshallErr != nil {
			errorResponse.Error = []*models.ErrorResponseErrorItems0{
				&models.ErrorResponseErrorItems0{
					Message: fmt.Sprintf("couldn't unmarshal json: %s\noriginal result was %#v", marshallErr, result),
				}}
			return graphql.NewWeaviateGraphqlPostUnprocessableEntity().WithPayload(errorResponse)
		}

		// Return the response
		return graphql.NewWeaviateGraphqlPostOK().WithPayload(graphQLResponse)
	})

	/*
	 * HANDLE BATCHING
	 */

	api.GraphqlWeaviateGraphqlBatchHandler = graphql.WeaviateGraphqlBatchHandlerFunc(func(params graphql.WeaviateGraphqlBatchParams, principal interface{}) middleware.Responder {
		defer messaging.TimeTrack(time.Now())
		messaging.DebugMessage("Starting GraphQL batch resolving")

		// Add security principal to context that we pass on to the GraphQL resolver.
		graphql_context := context.WithValue(params.HTTPRequest.Context(), "principal", (principal.(*models.KeyTokenGetResponse)))

		amountOfBatchedRequests := len(params.Body)
		errorResponse := &models.ErrorResponse{}

		if amountOfBatchedRequests == 0 {
			return graphql.NewWeaviateGraphqlBatchUnprocessableEntity().WithPayload(errorResponse)
		}
		requestResults := make(chan rest_api_utils.UnbatchedRequestResponse, amountOfBatchedRequests)

		wg := new(sync.WaitGroup)

		// Generate a goroutine for each separate request
		for requestIndex, unbatchedRequest := range params.Body {
			wg.Add(1)
			go handleUnbatchedGraphQLRequest(wg, graphql_context, unbatchedRequest, requestIndex, &requestResults)
		}

		wg.Wait()

		close(requestResults)

		batchedRequestResponse := make([]*models.GraphQLResponse, amountOfBatchedRequests)

		// Add the requests to the result array in the correct order
		for unbatchedRequestResult := range requestResults {
			batchedRequestResponse[unbatchedRequestResult.RequestIndex] = unbatchedRequestResult.Response
		}

		return graphql.NewWeaviateGraphqlBatchOK().WithPayload(batchedRequestResponse)
	})

	api.WeaviateBatchingActionsCreateHandler = operations.WeaviateBatchingActionsCreateHandlerFunc(func(params operations.WeaviateBatchingActionsCreateParams, principal interface{}) middleware.Responder {
		defer messaging.TimeTrack(time.Now())

		dbLock := db.ConnectorLock()
		requestLocks := rest_api_utils.RequestLocks{
			DBLock:      dbLock,
			DelayedLock: delayed_unlock.New(dbLock),
			DBConnector: dbLock.Connector(),
		}

		defer requestLocks.DelayedLock.Unlock()

		// Get context from request
		ctx := params.HTTPRequest.Context()

		// This is a write function, validate if allowed to write?
		if allowed, _ := auth.ActionsAllowed(ctx, []string{"write"}, principal, requestLocks.DBConnector, nil); !allowed {
			return operations.NewWeaviateBatchingActionsCreateForbidden()
		}

		amountOfBatchedRequests := len(params.Body.Actions)
		errorResponse := &models.ErrorResponse{}

		if amountOfBatchedRequests == 0 {
			return operations.NewWeaviateBatchingActionsCreateUnprocessableEntity().WithPayload(errorResponse)
		}

		isThingsCreate := false
		fieldsToKeep := determineResponseFields(params.Body.Fields, isThingsCreate)

		requestResults := make(chan rest_api_utils.BatchedActionsCreateRequestResponse, amountOfBatchedRequests)

		wg := new(sync.WaitGroup)

		async := params.Body.Async

		// Generate a goroutine for each separate request
		for requestIndex, batchedRequest := range params.Body.Actions {
			wg.Add(1)
			go handleBatchedActionsCreateRequest(wg, ctx, batchedRequest, requestIndex, &requestResults, async, principal, &requestLocks, fieldsToKeep)
		}

		wg.Wait()

		close(requestResults)

		batchedRequestResponse := make([]*models.ActionsGetResponse, amountOfBatchedRequests)

		// Add the requests to the result array in the correct order
		for batchedRequestResult := range requestResults {
			batchedRequestResponse[batchedRequestResult.RequestIndex] = batchedRequestResult.Response
		}

		return operations.NewWeaviateBatchingActionsCreateOK().WithPayload(batchedRequestResponse)

	})

	api.WeaviateBatchingThingsCreateHandler = operations.WeaviateBatchingThingsCreateHandlerFunc(func(params operations.WeaviateBatchingThingsCreateParams, principal interface{}) middleware.Responder {
		defer messaging.TimeTrack(time.Now())

		dbLock := db.ConnectorLock()
		requestLocks := rest_api_utils.RequestLocks{
			DBLock:      dbLock,
			DelayedLock: delayed_unlock.New(dbLock),
			DBConnector: dbLock.Connector(),
		}

		defer requestLocks.DelayedLock.Unlock()

		// Get context from request
		ctx := params.HTTPRequest.Context()

		// This is a write function, validate if allowed to write?
		if allowed, _ := auth.ActionsAllowed(ctx, []string{"write"}, principal, requestLocks.DBConnector, nil); !allowed {
			return operations.NewWeaviateBatchingThingsCreateForbidden()
		}

		amountOfBatchedRequests := len(params.Body.Things)
		errorResponse := &models.ErrorResponse{}

		if amountOfBatchedRequests == 0 {
			return operations.NewWeaviateBatchingThingsCreateUnprocessableEntity().WithPayload(errorResponse)
		}

		isThingsCreate := true
		fieldsToKeep := determineResponseFields(params.Body.Fields, isThingsCreate)

		requestResults := make(chan rest_api_utils.BatchedThingsCreateRequestResponse, amountOfBatchedRequests)

		wg := new(sync.WaitGroup)

		async := params.Body.Async

		// Generate a goroutine for each separate request
		for requestIndex, batchedRequest := range params.Body.Things {
			wg.Add(1)
			go handleBatchedThingsCreateRequest(wg, ctx, batchedRequest, requestIndex, &requestResults, async, principal, &requestLocks, fieldsToKeep)
		}

		wg.Wait()

		close(requestResults)

		batchedRequestResponse := make([]*models.ThingsGetResponse, amountOfBatchedRequests)

		// Add the requests to the result array in the correct order
		for batchedRequestResult := range requestResults {
			batchedRequestResponse[batchedRequestResult.RequestIndex] = batchedRequestResult.Response
		}

		return operations.NewWeaviateBatchingThingsCreateOK().WithPayload(batchedRequestResponse)
	})

	/*
	 * HANDLE KNOWLEDGE TOOLS
	 */
	api.KnowledgeToolsWeaviateToolsMapHandler = knowledge_tools.WeaviateToolsMapHandlerFunc(func(params knowledge_tools.WeaviateToolsMapParams, principal interface{}) middleware.Responder {
		return middleware.NotImplemented("operation knowledge_tools.WeaviateToolsMap has not yet been implemented")
	})

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// Handle a single unbatched GraphQL request, return a tuple containing the index of the request in the batch and either the response or an error
func handleUnbatchedGraphQLRequest(wg *sync.WaitGroup, ctx context.Context, unbatchedRequest *models.GraphQLQuery, requestIndex int, requestResults *chan rest_api_utils.UnbatchedRequestResponse) {
	defer wg.Done()

	// Get all input from the body of the request
	query := unbatchedRequest.Query
	operationName := unbatchedRequest.OperationName
	graphQLResponse := &models.GraphQLResponse{}

	// Return an unprocessable error if the query is empty
	if query == "" {

		// Regular error messages are returned as an error code in the request header, but that doesn't work for batched requests
		errorCode := strconv.Itoa(graphql.WeaviateGraphqlBatchUnprocessableEntityCode)
		errorMessage := fmt.Sprintf("%s: %s", errorCode, error422)
		errors := []*models.GraphQLError{&models.GraphQLError{Message: errorMessage}}
		graphQLResponse := models.GraphQLResponse{Data: nil, Errors: errors}
		*requestResults <- rest_api_utils.UnbatchedRequestResponse{
			requestIndex,
			&graphQLResponse,
		}
	} else {

		// Extract any variables from the request
		var variables map[string]interface{}
		if unbatchedRequest.Variables != nil {
			variables = unbatchedRequest.Variables.(map[string]interface{})
		}

		if graphQL == nil {
			panic("graphql is nil!")
		}
		result := graphQL.Resolve(query, operationName, variables, ctx)

		// Marshal the JSON
		resultJSON, jsonErr := json.Marshal(result)

		// Return an unprocessable error if marshalling the result to JSON failed
		if jsonErr != nil {

			// Regular error messages are returned as an error code in the request header, but that doesn't work for batched requests
			errorCode := strconv.Itoa(graphql.WeaviateGraphqlBatchUnprocessableEntityCode)
			errorMessage := fmt.Sprintf("%s: %s", errorCode, error422)
			errors := []*models.GraphQLError{&models.GraphQLError{Message: errorMessage}}
			graphQLResponse := models.GraphQLResponse{Data: nil, Errors: errors}
			*requestResults <- rest_api_utils.UnbatchedRequestResponse{
				requestIndex,
				&graphQLResponse,
			}
		} else {

			// Put the result data in a response ready object
			marshallErr := json.Unmarshal(resultJSON, graphQLResponse)

			// Return an unprocessable error if unmarshalling the result to JSON failed
			if marshallErr != nil {

				// Regular error messages are returned as an error code in the request header, but that doesn't work for batched requests
				errorCode := strconv.Itoa(graphql.WeaviateGraphqlBatchUnprocessableEntityCode)
				errorMessage := fmt.Sprintf("%s: %s", errorCode, error422)
				errors := []*models.GraphQLError{&models.GraphQLError{Message: errorMessage}}
				graphQLResponse := models.GraphQLResponse{Data: nil, Errors: errors}
				*requestResults <- rest_api_utils.UnbatchedRequestResponse{
					requestIndex,
					&graphQLResponse,
				}
			} else {

				// Return the GraphQL response
				*requestResults <- rest_api_utils.UnbatchedRequestResponse{
					requestIndex,
					graphQLResponse,
				}
			}
		}
	}

}

func handleBatchedActionsCreateRequest(wg *sync.WaitGroup, ctx context.Context, batchedRequest *models.ActionCreate, requestIndex int, requestResults *chan rest_api_utils.BatchedActionsCreateRequestResponse, async bool, principal interface{}, requestLocks *rest_api_utils.RequestLocks, fieldsToKeep map[string]int) {
	defer wg.Done()

	// Generate UUID for the new object
	UUID := connutils.GenerateUUID()

	// Validate schema given in body with the weaviate schema
	databaseSchema := schema.HackFromDatabaseSchema(requestLocks.DBLock.GetSchema())

	// Create Key-ref object
	url := serverConfig.GetHostAddress()
	keyRef := &models.SingleRef{
		LocationURL:  &url,
		NrDollarCref: principal.(*models.KeyTokenGetResponse).KeyID,
		Type:         string(connutils.RefTypeKey),
	}

	// Create Action object
	action := &models.Action{}
	action.AtContext = batchedRequest.AtContext
	action.LastUpdateTimeUnix = 0

	if _, ok := fieldsToKeep["@class"]; ok {
		action.AtClass = batchedRequest.AtClass
	}
	if _, ok := fieldsToKeep["schema"]; ok {
		action.Schema = batchedRequest.Schema
	}
	if _, ok := fieldsToKeep["creationtimeunix"]; ok {
		action.CreationTimeUnix = connutils.NowUnix()
	}
	if _, ok := fieldsToKeep["key"]; ok {
		action.Key = keyRef
	}

	// Create request result object
	result := &models.ActionsGetResponseAO1Result{}
	result.Errors = nil

	// Create request response object
	responseObject := &models.ActionsGetResponse{}
	responseObject.Action = *action
	if _, ok := fieldsToKeep["actionid"]; ok {
		responseObject.ActionID = UUID
	}
	responseObject.Result = result

	resultStatus := models.ActionsGetResponseAO1ResultStatusSUCCESS

	validatedErr := validation.ValidateActionBody(ctx, batchedRequest, databaseSchema, requestLocks.DBConnector,
		network, serverConfig, principal.(*models.KeyTokenGetResponse))

	if validatedErr != nil {
		// Edit request result status
		responseObject.Result.Errors = createErrorResponseObject(validatedErr.Error())
		resultStatus = models.ActionsGetResponseAO1ResultStatusFAILED
		responseObject.Result.Status = &resultStatus
	} else {
		// Handle asynchronous requests
		if async {
			requestLocks.DelayedLock.IncSteps()
			resultStatus = models.ActionsGetResponseAO1ResultStatusPENDING
			responseObject.Result.Status = &resultStatus

			go func() {
				defer requestLocks.DelayedLock.Unlock()
				err := requestLocks.DBConnector.AddAction(ctx, action, UUID)

				if err != nil {
					// Edit request result status
					resultStatus = models.ActionsGetResponseAO1ResultStatusFAILED
					responseObject.Result.Status = &resultStatus
					responseObject.Result.Errors = createErrorResponseObject(err.Error())
				}
			}()
		} else {
			// Handle synchronous requests
			err := requestLocks.DBConnector.AddAction(ctx, action, UUID)

			if err != nil {
				// Edit request result status
				resultStatus = models.ActionsGetResponseAO1ResultStatusFAILED
				responseObject.Result.Status = &resultStatus
				responseObject.Result.Errors = createErrorResponseObject(err.Error())
			}
		}
	}

	// Send this batched request's response and its place in the batch request to the channel
	*requestResults <- rest_api_utils.BatchedActionsCreateRequestResponse{
		requestIndex,
		responseObject,
	}
}

func handleBatchedThingsCreateRequest(wg *sync.WaitGroup, ctx context.Context, batchedRequest *models.ThingCreate, requestIndex int, requestResults *chan rest_api_utils.BatchedThingsCreateRequestResponse, async bool, principal interface{}, requestLocks *rest_api_utils.RequestLocks, fieldsToKeep map[string]int) {
	defer wg.Done()

	// Generate UUID for the new object
	UUID := connutils.GenerateUUID()

	// Validate schema given in body with the weaviate schema
	databaseSchema := schema.HackFromDatabaseSchema(requestLocks.DBLock.GetSchema())

	// Create Key-ref object
	url := serverConfig.GetHostAddress()
	keyRef := &models.SingleRef{
		LocationURL:  &url,
		NrDollarCref: principal.(*models.KeyTokenGetResponse).KeyID,
		Type:         string(connutils.RefTypeKey),
	}

	// Create Thing object
	thing := &models.Thing{}
	thing.AtContext = batchedRequest.AtContext
	thing.LastUpdateTimeUnix = 0

	if _, ok := fieldsToKeep["@class"]; ok {
		thing.AtClass = batchedRequest.AtClass
	}
	if _, ok := fieldsToKeep["schema"]; ok {
		thing.Schema = batchedRequest.Schema
	}
	if _, ok := fieldsToKeep["creationtimeunix"]; ok {
		thing.CreationTimeUnix = connutils.NowUnix()
	}
	if _, ok := fieldsToKeep["key"]; ok {
		thing.Key = keyRef
	}

	// Create request result object
	result := &models.ThingsGetResponseAO1Result{}
	result.Errors = nil

	// Create request response object
	responseObject := &models.ThingsGetResponse{}

	responseObject.Thing = *thing
	if _, ok := fieldsToKeep["thingid"]; ok {
		responseObject.ThingID = UUID
	}
	responseObject.Result = result

	resultStatus := models.ThingsGetResponseAO1ResultStatusSUCCESS

	validatedErr := validation.ValidateThingBody(ctx, batchedRequest, databaseSchema, requestLocks.DBConnector,
		network, serverConfig, principal.(*models.KeyTokenGetResponse))

	if validatedErr != nil {
		// Edit request result status
		responseObject.Result.Errors = createErrorResponseObject(validatedErr.Error())
		resultStatus = models.ThingsGetResponseAO1ResultStatusFAILED
		responseObject.Result.Status = &resultStatus
	} else {
		// Handle asynchronous requests
		if async {
			requestLocks.DelayedLock.IncSteps()
			resultStatus = models.ThingsGetResponseAO1ResultStatusPENDING
			responseObject.Result.Status = &resultStatus

			go func() {
				defer requestLocks.DelayedLock.Unlock()
				err := requestLocks.DBConnector.AddThing(ctx, thing, UUID)

				if err != nil {
					// Edit request result status
					resultStatus = models.ThingsGetResponseAO1ResultStatusFAILED
					responseObject.Result.Status = &resultStatus
					responseObject.Result.Errors = createErrorResponseObject(err.Error())
				}
			}()
		} else {
			// Handle synchronous requests
			err := requestLocks.DBConnector.AddThing(ctx, thing, UUID)

			if err != nil {
				// Edit request result status
				resultStatus = models.ThingsGetResponseAO1ResultStatusFAILED
				responseObject.Result.Status = &resultStatus
				responseObject.Result.Errors = createErrorResponseObject(err.Error())
			}
		}
	}

	// Send this batched request's response and its place in the batch request to the channel
	*requestResults <- rest_api_utils.BatchedThingsCreateRequestResponse{
		requestIndex,
		responseObject,
	}
}

// determine which field values not to return
func determineResponseFields(fields []*string, isThingsCreate bool) map[string]int {
	fieldsToKeep := map[string]int{"@class": 0, "schema": 0, "creationtimeunix": 0, "key": 0, "actionid": 0}

	// convert to things instead of actions
	if isThingsCreate {
		delete(fieldsToKeep, "actionid")
		fieldsToKeep["thingid"] = 0
	}

	if len(fields) > 0 {

		// check if "ALL" option is provided
		for _, field := range fields {
			fieldToKeep := strings.ToLower(*field)
			if fieldToKeep == "all" {
				return fieldsToKeep
			}
		}

		fieldsToKeep = make(map[string]int)
		// iterate over the provided fields
		for _, field := range fields {
			fieldToKeep := strings.ToLower(*field)
			fieldsToKeep[fieldToKeep] = 0
		}
	}

	return fieldsToKeep
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
	// Create message service
	messaging = &messages.Messaging{}

	// Load the config using the flags
	serverConfig = &config.WeaviateConfig{}
	err := serverConfig.LoadConfig(connectorOptionGroup, messaging)

	// Add properties to the config
	serverConfig.Hostname = addr
	serverConfig.Scheme = scheme

	// Fatal error loading config file
	if err != nil {
		messaging.ExitError(78, err.Error())
	}

	loadContextionary()

	connectToNetwork()

	// Connect to MQTT via Broker
	weaviateBroker.ConnectToMqtt(serverConfig.Environment.Broker.Host, serverConfig.Environment.Broker.Port)

	// Create the database connector usint the config
	err, dbConnector := dblisting.NewConnector(serverConfig.Environment.Database.Name, serverConfig.Environment.Database.DatabaseConfig)

	// Could not find, or configure connector.
	if err != nil {
		messaging.ExitError(78, err.Error())
	}

	// Construct a (distributed lock)
	localMutex := sync.RWMutex{}
	dbLock := database.RWLocker(&localMutex)

	// Configure schema manager
	if serverConfig.Environment.Database.LocalSchemaConfig == nil {
		messaging.ExitError(78, "Local schema manager is not configured.")
	}

	manager, err := db_local_schema_manager.New(
		serverConfig.Environment.Database.LocalSchemaConfig.StateDir, dbConnector, network)
	if err != nil {
		messaging.ExitError(78, fmt.Sprintf("Could not initialize local database state: %v", err))
	}

	manager.RegisterSchemaUpdateCallback(func(updatedSchema schema.Schema) {
		// Note that this is thread safe; we're running in a single go-routine, because the event
		// handlers are called when the SchemaLock is still held.

		fmt.Printf("UPDATESCHEMA DB: %#v\n", db)
		peers, err := network.ListPeers()
		if err != nil {
			graphQL = nil
			messaging.ErrorMessage(fmt.Sprintf("could not list network peers to regenerate schema:\n%#v\n", err))
			return
		}

		updatedGraphQL, err := graphqlapi.Build(&updatedSchema, peers, dbAndNetwork{Database: db, Network: network}, messaging)
		if err != nil {
			// TODO: turn on safe mode gh-520
			graphQL = nil
			messaging.ErrorMessage(fmt.Sprintf("Could not re-generate GraphQL schema, because:\n%#v\n", err))
		} else {
			messaging.InfoMessage("Updated GraphQL schema")
			graphQL = updatedGraphQL
		}
	})

	// Now instantiate a database, with the configured lock, manager and connector.
	err, db = database.New(messaging, dbLock, manager, dbConnector, contextionary)
	if err != nil {
		messaging.ExitError(1, fmt.Sprintf("Could not initialize the database: %s", err.Error()))
	}
	manager.TriggerSchemaUpdateCallbacks()

	network.RegisterUpdatePeerCallback(func(peers peers.Peers) {
		manager.TriggerSchemaUpdateCallbacks()
	})

	network.RegisterSchemaGetter(&schemaGetter{db: db})
}

type schemaGetter struct {
	db database.Database
}

func (s *schemaGetter) Schema() schema.Schema {
	dbLock := s.db.ConnectorLock()
	defer dbLock.Unlock()
	return dbLock.GetSchema()
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	// Rewrite / workaround because of issue with handling two API keys
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kth := keyTokenHeader{
			Key:   strfmt.UUID(r.Header.Get("X-API-KEY")),
			Token: strfmt.UUID(r.Header.Get("X-API-TOKEN")),
		}
		jkth, _ := json.Marshal(kth)
		r.Header.Set("X-API-KEY", string(jkth))
		r.Header.Set("X-API-TOKEN", string(jkth))

		messaging.InfoMessage("generated both headers X-API-KEY and X-API-TOKEN")

		handler.ServeHTTP(w, r)
	})
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	handleCORS := cors.New(cors.Options{
		OptionsPassthrough: true,
	}).Handler
	handler = handleCORS(handler)

	if feature_flags.EnableDevUI {
		handler = graphiql.AddMiddleware(handler)
		handler = swagger_middleware.AddMiddleware([]byte(SwaggerJSON), handler)
	}

	handler = addLogging(handler)
	handler = addPreflight(handler)

	return handler
}

func addLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serverConfig.Environment.Debug {
			log.Printf("Received request: %+v %+v\n", r.Method, r.URL)
		}
		next.ServeHTTP(w, r)
	})
}

func addPreflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "*")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// This function loads the Contextionary database, and creates
// an in-memory database for the centroids of the classes / properties in the Schema.
func loadContextionary() {
	// First load the file backed contextionary
	if serverConfig.Environment.Contextionary.KNNFile == "" {
		messaging.ExitError(78, "Contextionary KNN file not specified")
	}

	if serverConfig.Environment.Contextionary.IDXFile == "" {
		messaging.ExitError(78, "Contextionary IDX file not specified")
	}

	mmaped_contextionary, err := libcontextionary.LoadVectorFromDisk(serverConfig.Environment.Contextionary.KNNFile, serverConfig.Environment.Contextionary.IDXFile)

	if err != nil {
		messaging.ExitError(78, fmt.Sprintf("Could not load Contextionary; %+v", err))
	}

	messaging.InfoMessage("Contextionary loaded from disk")

	//TODO gh-618: update on schema change.
	//// Now create the in-memory contextionary based on the classes / properties.
	//databaseSchema :=
	//in_memory_contextionary, err := databaseSchema.BuildInMemoryContextionaryFromSchema(mmaped_contextionary)
	//if err != nil {
	//	messaging.ExitError(78, fmt.Sprintf("Could not build in-memory contextionary from schema; %+v", err))
	//}

	//// Combine contextionaries
	//contextionaries := []libcontextionary.Contextionary{*in_memory_contextionary, *mmaped_contextionary}
	//combined, err := libcontextionary.CombineVectorIndices(contextionaries)
	//
	// if err != nil {
	// 	messaging.ExitError(78, fmt.Sprintf("Could not combine the contextionary database with the in-memory generated contextionary; %+v", err))
	// }

	// messaging.InfoMessage("Contextionary extended with names in the schema")

	// // urgh, go.
	// x := libcontextionary.Contextionary(combined)
	// contextionary = &x

	// // whoop!

	contextionary = mmaped_contextionary
}

func connectToNetwork() {
	if serverConfig.Environment.Network == nil {
		messaging.InfoMessage(fmt.Sprintf("No network configured, not joining one"))
		network = libnetworkFake.FakeNetwork{}
	} else {
		genesis_url := strfmt.URI(serverConfig.Environment.Network.GenesisURL)
		public_url := strfmt.URI(serverConfig.Environment.Network.PublicURL)
		peer_name := serverConfig.Environment.Network.PeerName

		messaging.InfoMessage(fmt.Sprintf("Network configured, connecting to Genesis '%v'", genesis_url))
		new_net, err := libnetworkP2P.BootstrapNetwork(messaging, genesis_url, public_url, peer_name)
		if err != nil {
			messaging.ExitError(78, fmt.Sprintf("Could not connect to network! Reason: %+v", err))
		} else {
			network = *new_net
		}
	}
}
