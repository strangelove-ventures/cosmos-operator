pipeline {
    agent any
    stages {
        stage('test') {
            make test
        }
        stage('build') {
            make docker-build
        }
        stage('deploy') {
            make deploy
        }
    }
}