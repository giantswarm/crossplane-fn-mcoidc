package main

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"

	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/giantswarm/xfnlib/pkg/composite"

	"github.com/giantswarm/crossplane-fn-mcoidc/pkg/input/v1beta1"
)

const composedName = "crossplane-fn-mcoidc"

// RunFunction Execute the desired reconcilliation state, creating any required resources
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (rsp *fnv1.RunFunctionResponse, err error) {
	rsp = response.To(req, response.DefaultTTL)

	var (
		composed *composite.Composition
		input    v1beta1.Input
	)

	if input.Spec == nil {
		response.Fatal(rsp, &composite.MissingSpec{})
		return rsp, nil
	}

	oxr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrap(err, "cannot get observed composite resource"))
		return rsp, nil
	}

	if composed, err = composite.New(req, &input, &oxr); err != nil {
		response.Fatal(rsp, errors.Wrap(err, "error setting up function "+composedName))
		return rsp, nil
	}

	if err = f.DiscoverAccounts(input.Spec.AWSAccountsPatchToRef, composed); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot discover accounts"))
		return rsp, nil
	}

	if err = composed.ToResponse(rsp); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert composition to response %T", rsp))
		return
	}

	return rsp, nil
}

func (f *Function) patchFieldValueToObject(fieldPath string, value any, to runtime.Object) (err error) {
	var paved *fieldpath.Paved
	if paved, err = fieldpath.PaveObject(to); err != nil {
		return
	}

	if err = paved.SetValue(fieldPath, value); err != nil {
		return
	}

	return runtime.DefaultUnstructuredConverter.FromUnstructured(paved.UnstructuredContent(), to)
}
