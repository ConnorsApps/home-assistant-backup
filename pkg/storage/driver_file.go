//go:build file || (!s3 && !gcs)

package storage

import _ "gocloud.dev/blob/fileblob"
