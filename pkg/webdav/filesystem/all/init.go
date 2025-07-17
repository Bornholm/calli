package all

import (
	_ "github.com/bornholm/calli/pkg/webdav/filesystem/cor"
	_ "github.com/bornholm/calli/pkg/webdav/filesystem/local"
	_ "github.com/bornholm/calli/pkg/webdav/filesystem/s3"
	_ "github.com/bornholm/calli/pkg/webdav/filesystem/sqlite"
)
