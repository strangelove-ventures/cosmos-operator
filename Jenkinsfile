pipeline {
  agent any
  stages {
    stage('verify make is installed') {
      steps {
        sh 'make --version'
      }
    }
    stage('make tools') {
      environment {
        PATH="$PATH:/usr/local/go/bin"
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make tools'
      }
    }
    stage('make test') {
      environment {
        PATH="$PATH:/usr/local/go/bin"
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make test'
      }
    }
    stage('make build') {
      environment {
        PATH="$PATH:/usr/local/go/bin"
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make docker-build'
      }
    }
    stage('make deploy') {
      environment {
        PATH="$PATH:/usr/local/go/bin"
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
        KUBECONFIG="/.kube/config"
      }
      steps {
        sh 'make deploy'
      }
    }
  }
}
