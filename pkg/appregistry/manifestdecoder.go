package appregistry

import (
	"fmt"

	"github.com/kevinrizza/offline-cataloger/pkg/apprclient"
	// "github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// NewManifestDecoder creates a new manifestDecoder
func NewManifestDecoder() (*ManifestDecoder, error) {
	bundle, err := NewBundleProcessor()
	if err != nil {
		return nil, err
	}

	flattened, err := NewFlattenedProcessor()
	if err != nil {
		return nil, err
	}

	return &ManifestDecoder{
		flattened: flattened,
		nested:    bundle,
		walker:    &tarWalker{},
	}, nil
}

type result struct {
	// FlattenedCount is the total number of flattened single-file operator
	// manifest(s) processed so far.
	FlattenedCount int

	// NestedCount is the total number of nested operator manifest(s)
	// processed so far.
	NestedCount int
}

// IsEmpty returns true if no operator manifest has been processed so far.
func (r *result) IsEmpty() bool {
	return r.FlattenedCount == 0 && r.NestedCount == 0
}

type ManifestDecoder struct {
	//logger    *logrus.Entry
	flattened *flattenedProcessor
	nested    *bundleProcessor
	walker    *tarWalker
	directory string
}

// Decode iterates through each operator manifest blob that is encoded in a tar
// ball and processes it accordingly.
//
// On return, result.Flattened is populates with the set of operator manifest(s)
// specified in flattened format ( one file with data section). If there are no
// operator(s) in flattened format result.Flattened is set to nil
//
// On return, result.NestedDirectory is set to the path of the folder which
// contains operator manifest(s) specified in nested bundle format. If there are
// no operator(s) in nested bundle format then result.NestedCount  is set to
// zero.
//
// This function takes a best-effort approach. On return, err is set to an
// aggregated list of error(s) encountered. The caller should inspect the
// result object to determine the next steps.
func (d *ManifestDecoder) Decode(manifests []*apprclient.OperatorMetadata, workingDirectory string) (result result, err error) {
	getProcessor := func(isNested bool) (Processor, string) {
		if isNested {
			return d.nested, "nested"
		}

		return d.flattened, "flattened"
	}

	fmt.Println()

	allErrors := []error{}
	for _, om := range manifests {
		fmt.Println(fmt.Sprintf("repository: %s", om.RegistryMetadata.String()))

		// Determine the format type of the manifest blob and select the right processor.
		checker := NewFormatChecker()
		walkError := d.walker.Walk(om.Blob, om.RegistryMetadata.Name, workingDirectory, checker)
		if walkError != nil {
			fmt.Println(fmt.Sprintf("skipping, can't determine the format of the manifest - %v", walkError))
			allErrors = append(allErrors, err)
			continue
		}

		if checker.IsNestedBundleFormat() {
			result.NestedCount++
		}

		processor, format := getProcessor(checker.IsNestedBundleFormat())
		fmt.Println(fmt.Sprintf("manifest format is - %s", format))

		walkError = d.walker.Walk(om.Blob, om.RegistryMetadata.Name, workingDirectory, processor)
		if walkError != nil {
			fmt.Println(fmt.Sprintf("skipping due to error - %v", walkError))
			allErrors = append(allErrors, err)
			continue
		}

		fmt.Println(fmt.Sprintf("decoded successfully"))
	}

	result.FlattenedCount = d.flattened.GetProcessedCount()

	err = utilerrors.NewAggregate(allErrors)
	return
}
