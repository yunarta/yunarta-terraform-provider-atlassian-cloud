package test

import (
	"bytes"
	"github.com/gorilla/mux"
	"github.com/yunarta/terraform-api-transport/transport"
	"io"
	"net/http"
	"net/http/httptest"
)

type JiraTransport struct {
	router *mux.Router
}

var _ transport.PayloadTransport = &JiraTransport{}

func (j *JiraTransport) SendWithExpectedStatus(request *transport.PayloadRequest, expectedStatus ...int) (*transport.PayloadResponse, error) {
	return j.Send(request)
}

func (j *JiraTransport) Send(request *transport.PayloadRequest) (*transport.PayloadResponse, error) {
	var reader io.Reader
	if request.Payload != nil {
		reader = bytes.NewReader(request.Payload.ContentMust())
	} else {
		reader = nil
	}

	muxRequest, err := http.NewRequest(
		request.Method,
		request.Url,
		reader,
	)
	if err != nil {
		return nil, err
	}

	muxResponse := httptest.NewRecorder()
	j.router.ServeHTTP(muxResponse, muxRequest)
	return &transport.PayloadResponse{
		StatusCode: muxResponse.Code,
		Body:       muxResponse.Body.String(),
	}, nil
}

func NewJiraTransport() *JiraTransport {
	router := mux.NewRouter()
	router.HandleFunc("/rest/api/latest/user/search", userSearchHandler)

	return &JiraTransport{
		router: router,
	}
}

func userSearchHandler(writer http.ResponseWriter, request *http.Request) {
	writer.WriteHeader(200)
	//query := request.URL.Query().Read("query")

	_, _ = writer.Write([]byte(`[{"self":"https://mobilesolutionworks.atlassian.net/rest/api/3/user?accountId=557058:32b276cf-1a9f-45ae-b3f5-f850bc24f1b9","accountId":"557058:32b276cf-1a9f-45ae-b3f5-f850bc24f1b9","accountType":"atlassian","emailAddress":"yunarta.kartawahyudi@gmail.com","avatarUrls":{"48x48":"https://secure.gravatar.com/avatar/a607ac1755019f3fd32eb16294c81292?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYK-6.png","24x24":"https://secure.gravatar.com/avatar/a607ac1755019f3fd32eb16294c81292?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYK-6.png","16x16":"https://secure.gravatar.com/avatar/a607ac1755019f3fd32eb16294c81292?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYK-6.png","32x32":"https://secure.gravatar.com/avatar/a607ac1755019f3fd32eb16294c81292?d=https%3A%2F%2Favatar-management--avatars.us-west-2.prod.public.atl-paas.net%2Finitials%2FYK-6.png"},"displayName":"Yunarta Kartawahyudi","active":true,"timeZone":"Etc/GMT","locale":"en_US"}]`))
}
