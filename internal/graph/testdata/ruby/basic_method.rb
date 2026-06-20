class UsersController < ApplicationController
  def index
    if params[:admin]
      @users = User.admins
    else
      @users = User.all
    end
  end
end
