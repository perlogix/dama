# dama
A simplified machine learning container platform that helps teams get started with an automated workflow.

![demo gif](https://taskfit.io/dama-demo.gif)

[Larger Demo Video](https://taskfit.io/dama-demo-2.gif)

### DISCLAIMER: dama is currently in alpha due to the lack of security and scaling, but still fun to try out!

## Server Configuration
**Server default configurations in config.yml**
These configurations are loaded by default if not overridden in `config.yml`.

	expire: "1300"
	deployexpire: "86400"
	uploadsize: 2000000000
	envsize: 20
	https:
	  listen: "0.0.0.0"
	  port: "8443"
	  debug: false
	  verifytls: false
	db:
	  db: 0
	  maxretries: 20
	docker:
	  endpoint: "unix:///var/run/docker.sock"
	  cpushares: 512
	  memory: 1073741824
	gotty:
	  tls: false

These configurations need to be set in your environment variables.

	# Server admin username and password
	DamaUser       # example: DamaUser="tim"
    DamaPassword   # example: DamaPassword="9e9692478ca848a19feb8e24e5506ec89"

	# Redis database password if applicable
	DBPassword     # example: DBPassword="9e9692478ca848a19feb8e24e5506ec89"

All configurations types

	images: ["taskfit:minimal"]                # required / string array
	expire: "1300"                             # string
	deployexpire: "86400"                      # string
	uploadsize: 2000000000                     # int
	envsize: 20                                # int
	https:
	  listen: "0.0.0.0"                        # string
	  port: "8443"                             # string
	  pem: "/opt/dama.pem"                     # required / string
	  key: "/opt/dama.key"                     # required / string
	  debug: false                             # bool
	  verifytls: false                         # bool
	db:
	  network: "unix"                          # required / string
	  address: "./tmp/redis.sock"              # required / string
	  db: 0                                    # int
	  maxretries: 20                           # int
	docker:
	  endpoint: "unix:///var/run/docker.sock"  # string
	  cpushares: 512                           # int
	  memory: 1073741824                       # int
	gotty:
	  tls: false                               # bool

## CLI Configuration
These environment variables need to be exported in order to use dama-cli.

    DAMA_SERVER # example: export DAMA_SERVER="https://localhost:8443/"
    DAMA_USER   # example: export DAMA_USER="tim"
    DAMA_KEY    # example: export DAMA_KEY="9e9692478ca848a19feb8e24e5506ec89"

## CLI Flags
	Usage: dama [options] <args>

	 -new           Create a new environment from scratch and delete the old one
	 -run           Create environment and run with dama.yml
	 -file          Run with dama.yml in different directory
	 -env           Create an environment variable or secret for runtime
	 -img           Specify a docker image to be used instead of the default image
	 -dl            Download file from workspace in your environment to your local computer
	 -up            Upload files from your local computer to workspace in your environment
	 -deploy        Deploy API and get your unique URI
	 -show-api      Show API details: URL, Health and Type
	 -show-images   Show images available to use

## CLI Examples
	dama -new
	dama -run
	dama -run -file ../dama.yml
	dama -env "AWS_ACCESS_KEY_ID=123,AWS_SECRET_ACCESS_KEY=234"
	dama -deploy
	dama -run -img tensorflow:lite
	dama -show-images
	dama -show-api
	dama -up data.csv
	dama -dl model.pkl

## dama.yml File
This a simple `dama.yml` to setup your environment and run a Flask API.

	image: "taskfit:minimal"
	port: "5000"
	pip: |
	  Flask==0.12.2
	  scikit-learn==0.19.1
	  numpy==1.14.2
	  scipy==1.0.0
	python: |
	  from flask import Flask, request, jsonify
	  from sklearn import datasets
	  from sklearn.model_selection import train_test_split
	  from sklearn.ensemble import RandomForestClassifier
	  from sklearn.externals import joblib

	  X, y = datasets.load_iris(return_X_y=True)
	  X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.3, random_state=42)
	  model = RandomForestClassifier(random_state=101)
	  model.fit(X_train, y_train)
	  print("Score on the training set is: {:2}".format(model.score(X_train, y_train)))
	  print("Score on the test set is: {:.2}".format(model.score(X_test, y_test)))
	  model_filename = 'iris-rf-v1.0.pkl'
	  print("Saving model to {}...".format(model_filename))
	  joblib.dump(model, model_filename)
	  app = Flask(__name__)
	  MODEL = joblib.load('iris-rf-v1.0.pkl')
	  MODEL_LABELS = ['setosa', 'versicolor', 'virginica']

	  @app.route('/predict')
	    def predict():
	      sepal_length = request.args.get('sepal_length')
	      sepal_width = request.args.get('sepal_width')
	      petal_length = request.args.get('petal_length')
	      petal_width = request.args.get('petal_width')
	      features = [[sepal_length, sepal_width, petal_length, petal_width]]
	      label_index = MODEL.predict(features)
	      label = MODEL_LABELS[label_index[0]]
	      return jsonify(status='complete', label=label)
		
	  if __name__ == '__main__':
	    app.run(debug=False, host="0.0.0.0", threaded=True)

cURL API in sandbox or deploy

    curl -ks https://localhost:8443/api/<insert sandbox key>/predict?sepal_length=5&sepal_width=3.1&petal_length=2.5&petal_width=1.2

Even simpler environment setup with model training.

	image: "taskfit:tensorflow"
	checkout: "https://github.com/aymericdamien/TensorFlow-Examples.git"
	cmd: |
	  cd TensorFlow-Examples/examples/3_NeuralNetworks
	  python neural_network.py

All YAML configuration option types.

	project         # string       - proejct name
	env             # string array - env variables
	checkout        # string       - git checkout master branch
	time_format     # string       - python time format used in container as env variable TIMESTAMP
	setup_cmd       # string       - run setup /initial command before cmd or python
	cmd             # string       - run BASH Linux command
	python          # string       - run inline Python
	pip             # string       - install pip packages
	image           # string       - define container image for environment
	port            # string       - port to expose for web service
	git:
	  url           # string       - git URL
	  branch        # string       - git branch
	  sha           # string       - git SHA
	aws_s3:
	  file          # string       - file to push or pull
	  dir           # string       - directory to push or pull
	  bucket_push   # string       - push file or dir to S3
	  bucket_pull   # string       - pull file or dir from S3

## Dockerfiles
Add these lines to your Dockerfiles for your CLI to connect via websockets

    RUN cd /usr/bin && curl -L https://github.com/yudai/gotty/releases/download/v1.0.1/gotty_linux_amd64.tar.gz | tar -xz
    CMD ["/usr/bin/gotty", "--reconnect", "-w", "/bin/bash"]

## Build

    make build

## To Do

 - [ ] Tokenize environment variables in DB
 - [ ] Write test suite
 - [ ] Provide Vagrant and Docker images
 - [ ] Add scheduler / resource manager for multi-host container serving
 - [ ] Rewrite auth middleware
 - [ ] Swap out stdlib flags package for third-party package
 - [ ] These docs stink!