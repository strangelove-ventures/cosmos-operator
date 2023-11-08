pipeline {
  agent any
  stages {
    stage('verify make is installed') {
      steps {
        sh 'make --version'
      }
    }
    stage('make tools') {
      steps {
        sh 'make tools'
      }
    }
    stage('make test') {
      steps {
        sh 'make test'
      }
    }
    stage('make build') {
      steps {
        sh 'make docker-build'
      }
    }
    stage('make deploy') {
      steps {
        sh 'make deploy'
      }
    }
  }
}

