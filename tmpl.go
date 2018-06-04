package main

// tmpl is a text/template to create a bash script in /root/workspace/.dama
var tmpl = `#!/bin/bash
if [[ ! -f /root/.taskfit ]]; then
touch /root/.taskfit
{{if .SetupCmd}}
{{ .SetupCmd }}
{{- end}}
{{if .Pip}}
pip install {{ .Pip }}
{{- end}}
{{if .Checkout}}
git clone {{.Checkout}}
{{- end}}
{{if .Git.URL}}
{{if .Git.Branch}}
git clone {{.Git.URL}} -b {{.Git.Branch}}
{{else}}
git clone {{.Git.URL}}
{{- end}}
{{- end}}
{{if .AWS_S3.BucketPull}}
aws s3 sync {{.AWS_S3.BucketPull}} .
{{- end}}
{{ .Cmd }}
{{if .Python}}
python << EOF
{{ .Python }}
EOF
{{- end}}
{{if and .AWS_S3.File .AWS_S3.BucketPush}}
aws s3 cp {{.AWS_S3.File}} {{.AWS_S3.BucketPush}}
{{- end}}
{{if and .AWS_S3.Dir .AWS_S3.BucketPush}}
aws s3 sync {{.AWS_S3.Dir}} {{.AWS_S3.BucketPush}}
{{- end}}
fi
bash`
