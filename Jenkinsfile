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
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make tools'
      }
    }
    stage('make test') {
      environment {
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make test'
      }
    }
    stage('make build') {
      environment {
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make docker-build'
      }
    }
    stage('make deploy') {
      environment {
        GOROOT="/usr/local/go"
        GOPATH="/usr/local/go/bin"
      }
      steps {
        sh 'make deploy'
      }
    }
  }
}
