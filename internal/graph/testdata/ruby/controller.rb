class UsersController < ApplicationController
  def index
    @users = User.all
  end

  def show
    @user = User.find(params[:id])
  end

  def create
    if user_params[:name].empty?
      render plain: "Name required", status: 422
    else
      @user = User.new(user_params)
      @user.save
    end
  end
end
