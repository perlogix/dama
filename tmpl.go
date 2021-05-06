package main

// tmpl is a text/template to create a bash script in /root/workspace/.dama
var tmpl = `#!/bin/bash
if [[ ! -f /root/.dama ]]; then
touch /root/.dama
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
{{if .AWSs3.BucketPull}}
aws s3 sync {{.AWSs3.BucketPull}} .
{{- end}}
{{ .Cmd }}
{{if .Python}}
python << EOF
{{ .Python }}
EOF
{{- end}}
{{if and .AWSs3.File .AWSs3.BucketPush}}
aws s3 cp {{.AWSs3.File}} {{.AWSs3.BucketPush}}
{{- end}}
{{if and .AWSs3.Dir .AWSs3.BucketPush}}
aws s3 sync {{.AWSs3.Dir}} {{.AWSs3.BucketPush}}
{{- end}}
fi
bash`
