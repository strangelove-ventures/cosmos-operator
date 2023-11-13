pipeline {
  agent any
  environment {
      LOCALBIN="/var/jenkins_home/workspace/dasfs-test/bin"
  }
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
    stage('git clone repository') {
      steps {
        git branch: 'main', credentialsId: 'jinu-github', url: 'https://github.com/qj0r9j0vc2/test-argocd'
      }
    }
    stage('edit repository') {
      steps {
        sh "rm nginx-deploy.yaml"
        sh 'make deploy > nginx-deploy.yaml'
      }
    }
    stage('push git') {
      steps {
        sh "git push origin main"
      }
    }
    
  }
}


