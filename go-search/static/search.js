/*
*/

function TaskCtrl($scope, $http) {
  $scope.tasks = [];
  $scope.working = false;
  $scope.results = [];
  $scope.lastquery = '';

  var logError = function(data, status) {
    console.log('code '+status+': '+data);
    $scope.working = false;
  };

  var refresh = function() {
    return $http.post('/search/', {Query: $scope.lastquery}).
      success(function(data) { 
        if($scope.lastquery.length() < 1)
          $scope.results = [];
        else
        {
		  $scope.results = data.Results;
              console.log(data.Results);
        }
      }).
      error(logError);
  };

  $scope.addTodo = function() {
    $scope.working = true;
    $http.post('/search/', {Query: $scope.todoText}).
      error(logError).
      success(function(data) {
        $scope.results = data.Results;
        $scope.lastquery = $scope.todoText
        $scope.working = false;
        $scope.todoText = '';
      });
  };

  $scope.toggleDone = function(task) {
    data = {ID: task.ID, Title: task.Title, Done: !task.Done}
    $http.put('//'+task.ID, data).
      error(logError).
      success(function() { task.Done = !task.Done });
  };

  refresh().then(function() { $scope.working = false; });
}
